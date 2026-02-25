package shell

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	cmap "github.com/orcaman/concurrent-map/v2"
	"golang.org/x/term"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	trcshmemfs "github.com/trimble-oss/tierceron-core/v2/trcshfs"
	"github.com/trimble-oss/tierceron-core/v2/trcshfs/trcshio"
	testr "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcrosea/rosea/editor"

	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trctrcsh/dirpicker"
)

var (
	promptStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	outputStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	chatMsgHooks = cmap.New[tccore.ChatHookFunc]()
)

// commandResultMsg is sent when a command completes execution
type commandResultMsg struct {
	output     []string
	shouldQuit bool
}

// editorCloseMsg is sent when the editor wants to close
type editorCloseMsg struct{}

// dirPickerCompleteMsg is sent when dirpicker completes with a selection
type dirPickerCompleteMsg struct {
	selectedPath string
	cancelled    bool
}

// dirPickerStartMsg is sent to enter dirpicker mode
type dirPickerStartMsg struct {
	startPath  string
	pendingCmd string
}

// GetChatMsgHooks returns the chat message hooks map
func GetChatMsgHooks() *cmap.ConcurrentMap[string, tccore.ChatHookFunc] {
	return &chatMsgHooks
}

type ShellModel struct {
	width              int
	height             int
	prompt             string
	input              string
	cursor             int
	cursorVisible      bool // For blinking cursor
	history            []string
	historyIndex       int
	draft              string
	output             []string       // Persistent buffer - holds ALL output
	viewport           viewport.Model // Viewport handles scrolling
	memFs              trcshio.MemoryFileSystem
	chatSenderChan     *chan *tccore.ChatMsg
	pendingExit        bool
	elevatedMode       bool                      // Track if user has unrestricted write access
	commandExecuting   bool                      // Track if a command is currently executing
	editorModel        tea.Model                 // Active editor model (nil when not editing)
	dirPickerMode      bool                      // Track if dirpicker is active
	dirPicker          *dirpicker.DirPickerModel // Active dirpicker instance
	pendingCommand     string                    // Command waiting for dirpicker selection
	selectionStart     int                       // Start position for text selection
	selectionEnd       int                       // End position for text selection
	isSelecting        bool                      // Flag to track if user is selecting
	lastMemClipTime    time.Time                 // Timestamp of last memFs clipboard update
	lastSysClipTime    time.Time                 // Timestamp when system clipboard content was detected
	lastSysClipContent string                    // Last system clipboard content we saw
}

func InitShell(chatSenderChan *chan *tccore.ChatMsg, memFs ...trcshio.MemoryFileSystem) *ShellModel {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width = 80
		height = 24
	}

	var memFileSystem trcshio.MemoryFileSystem
	if len(memFs) > 0 && memFs[0] != nil {
		memFileSystem = memFs[0]
	} else {
		memFileSystem = trcshmemfs.NewTrcshMemFs()
	}

	// Initialize viewport for scrolling
	// Reserve 3 lines: 1 for blank line, 1 for prompt, 1 for input
	vp := viewport.New(width, height-3)
	initialOutput := []string{"Welcome to trcsh interactive shell", "Type 'help' for available commands, 'exit' or Ctrl+C to quit", ""}
	vp.SetContent(strings.Join(initialOutput, "\n"))

	return &ShellModel{
		width:              width,
		height:             height,
		prompt:             "$",
		input:              "",
		cursor:             0,
		cursorVisible:      true,
		history:            []string{},
		historyIndex:       -1,
		draft:              "",
		output:             initialOutput,
		viewport:           vp,
		memFs:              memFileSystem,
		chatSenderChan:     chatSenderChan,
		pendingExit:        false,
		elevatedMode:       false,
		commandExecuting:   false,
		editorModel:        nil,
		dirPickerMode:      false,
		dirPicker:          nil,
		pendingCommand:     "",
		selectionStart:     -1,
		selectionEnd:       -1,
		isSelecting:        false,
		lastMemClipTime:    time.Now(),
		lastSysClipTime:    time.Time{}, // Start with zero time
		lastSysClipContent: "",          // Start empty
	}
}

// cursorBlinkMsg triggers cursor blink
type cursorBlinkMsg struct{}

func cursorBlink() tea.Cmd {
	return tea.Tick(time.Millisecond*530, func(t time.Time) tea.Msg {
		return cursorBlinkMsg{}
	})
}

func (m *ShellModel) Init() tea.Cmd {
	return cursorBlink()
}

// copyToMemFsClipboard stores content in memFs clipboard with retry logic
func (m *ShellModel) copyToMemFsClipboard(content string) {
	if m.memFs == nil {
		return
	}

	// Retry up to 3 times to write to clipboard
	for attempt := 0; attempt < 3; attempt++ {
		// Only remove if file exists (skip on attempt 0 since it won't exist)
		if attempt > 0 {
			m.memFs.Remove("/.clipboard")
		}

		// Create new clipboard file and write content
		file, err := m.memFs.Create("/.clipboard")
		if err != nil {
			// Retry on error
			continue
		}

		_, err = file.Write([]byte(content))
		file.Close()

		if err == nil {
			// Success - update timestamp
			m.lastMemClipTime = time.Now()
			return
		}
		// Retry on write error
	}
}

// readMemFsClipboard retrieves content from memFs clipboard
func (m *ShellModel) readMemFsClipboard() (string, time.Time) {
	if m.memFs == nil {
		return "", m.lastMemClipTime
	}
	file, err := m.memFs.Open("/.clipboard")
	if err != nil {
		return "", m.lastMemClipTime
	}
	defer file.Close()

	// Read file content
	var buf strings.Builder
	_, err = io.Copy(&buf, file)
	if err != nil {
		return "", m.lastMemClipTime
	}
	return buf.String(), m.lastMemClipTime
}

// readSystemClipboard attempts to read from system clipboard using native tools
// Only updates timestamp when content actually changes (not every read)
// Tries xclip, xsel, wl-paste, and WSL powershell depending on what's available
// Returns empty string if not available
func (m *ShellModel) readSystemClipboard() (string, time.Time) {
	var content string

	// Try X11 with xclip
	if c, err := tryX11ClipboardXclip(); err == nil && c != "" {
		content = c
	} else if c, err := tryX11ClipboardXsel(); err == nil && c != "" {
		// Try X11 with xsel
		content = c
	} else if c, err := tryWaylandClipboard(); err == nil && c != "" {
		// Try Wayland (wl-paste)
		content = c
	} else if c, err := tryWSLClipboard(); err == nil && c != "" {
		// Try WSL (PowerShell Get-Clipboard)
		content = c
	}

	// Only update timestamp if content is new (different from what we last saw)
	if content != "" && content != m.lastSysClipContent {
		m.lastSysClipTime = time.Now()
		m.lastSysClipContent = content
	}

	return content, m.lastSysClipTime
}

// tryX11ClipboardXclip tries to get clipboard from X11 using xclip
func tryX11ClipboardXclip() (string, error) {
	cmd := exec.Command("xclip", "-selection", "clipboard", "-o")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return out.String(), nil
}

// tryX11ClipboardXsel tries to get clipboard from X11 using xsel
func tryX11ClipboardXsel() (string, error) {
	cmd := exec.Command("xsel", "--clipboard", "--output")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return out.String(), nil
}

// tryWaylandClipboard tries to get clipboard from Wayland
func tryWaylandClipboard() (string, error) {
	cmd := exec.Command("wl-paste", "--no-newline")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return out.String(), nil
}

// tryWSLClipboard tries to get clipboard from WSL using PowerShell
func tryWSLClipboard() (string, error) {
	cmd := exec.Command("powershell.exe", "-command", "Get-Clipboard")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	content := out.String()
	// Remove Windows line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimRight(content, "\n")
	return content, nil
}

// getClipboardToUse returns content from the most recently updated clipboard
// Prefers memFs if it's newer, otherwise tries system clipboard
func (m *ShellModel) getClipboardToUse() string {
	memContent, memTime := m.readMemFsClipboard()
	sysContent, sysTime := m.readSystemClipboard()

	// If system clipboard is empty, use memFs
	if sysContent == "" {
		return memContent
	}
	// If memFs is empty, use system
	if memContent == "" {
		return sysContent
	}
	// Both have content - use newer one
	if memTime.After(sysTime) {
		return memContent
	}
	return sysContent
}

// getSelectedText returns the text that is currently selected
func (m *ShellModel) getSelectedText() string {
	if m.selectionStart < 0 || m.selectionEnd < 0 {
		return ""
	}

	// Flatten output to single string with newlines
	fullText := strings.Join(m.output, "\n")

	start := m.selectionStart
	end := m.selectionEnd
	if start > end {
		start, end = end, start
	}

	if start < 0 || end > len(fullText) {
		return ""
	}

	return fullText[start:end]
}

// clearSelection resets selection
func (m *ShellModel) clearSelection() {
	m.selectionStart = -1
	m.selectionEnd = -1
	m.isSelecting = false
}

func (m *ShellModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle dirpicker start messages - enter dirpicker mode
	if startMsg, ok := msg.(dirPickerStartMsg); ok {
		m.dirPickerMode = true
		m.dirPicker = dirpicker.NewDirPicker(startMsg.startPath)
		m.pendingCommand = startMsg.pendingCmd
		return m, nil
	}

	// If dirpicker is active, forward ALL messages to it
	if m.dirPickerMode && m.dirPicker != nil {
		updated, cmd := m.dirPicker.Update(msg)
		m.dirPicker = updated.(*dirpicker.DirPickerModel)

		// Check if dirpicker is done
		if m.dirPicker.Selected() || m.dirPicker.Cancelled() {
			m.dirPickerMode = false

			if m.dirPicker.Cancelled() {
				// User cancelled - just return to prompt
				m.pendingCommand = ""
				m.dirPicker = nil
				m.commandExecuting = false
				return m, nil
			}

			// User selected a directory - modify the pending command to replace -ofs with -outputDir=path
			selectedPath := m.dirPicker.CurrentPath()
			if m.pendingCommand != "" {
				parts := strings.Fields(m.pendingCommand)
				modifiedArgs := make([]string, 0, len(parts))

				// Keep all parts except -ofs
				for _, part := range parts {
					if part != "-ofs" {
						modifiedArgs = append(modifiedArgs, part)
					}
				}

				// Add -outputDir with selected path
				modifiedArgs = append(modifiedArgs, fmt.Sprintf("-outputDir=%s", selectedPath))

				// Create modified command string and execute it
				modifiedCmd := strings.Join(modifiedArgs, " ")
				m.pendingCommand = ""

				// Add output line showing what we're executing
				m.output = append(m.output, promptStyle.Render(m.prompt+" ")+modifiedCmd)
				m.updateViewportContent()
				m.viewport.GotoBottom()

				m.dirPicker = nil
				return m, m.executeCommandAsync(modifiedCmd)
			}
			m.dirPicker = nil
			return m, nil
		}

		// Still picking - return dirpicker's command
		return m, cmd
	}

	// Handle editor close messages first
	if _, ok := msg.(testr.EditorCloseMsg); ok {
		// Editor requested to close - follow same pattern as commandResultMsg
		m.editorModel = nil
		m.commandExecuting = false
		wasAtBottom := m.viewport.AtBottom()
		m.updateViewportContent()
		if wasAtBottom {
			m.viewport.GotoBottom()
		}
		return m, cursorBlink()
	}
	if _, ok := msg.(editorCloseMsg); ok {
		// Legacy - can remove later
		m.editorModel = nil
		m.commandExecuting = false
		wasAtBottom := m.viewport.AtBottom()
		m.updateViewportContent()
		if wasAtBottom {
			m.viewport.GotoBottom()
		}
		return m, cursorBlink()
	}

	// If editor is active, forward ALL messages to it
	if m.editorModel != nil {
		updated, cmd := m.editorModel.Update(msg)
		m.editorModel = updated
		return m, cmd
	}

	// Shell-only message handling (when editor is not active)
	switch msg := msg.(type) {
	case cursorBlinkMsg:
		m.cursorVisible = !m.cursorVisible
		return m, cursorBlink()

	case tea.QuitMsg:
		// Ignore QuitMsg - shell uses explicit exit handling
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update viewport size (reserve 3 lines for prompt area)
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 3
		return m, nil

	case tea.MouseMsg:
		// Handle text selection via mouse
		if msg.Type == tea.MouseLeft {
			// Get position adjusted for viewport scrolling
			fullText := strings.Join(m.output, "\n")
			lines := strings.Split(fullText, "\n")

			// Account for viewport's YOffset (scrolling)
			adjustedY := msg.Y + m.viewport.YOffset
			x := msg.X

			if adjustedY >= 0 && adjustedY < len(lines) {
				line := lines[adjustedY]
				if x >= 0 && x <= len(line) {
					// Calculate position in flattened string
					pos := 0
					for i := 0; i < adjustedY; i++ {
						pos += len(lines[i]) + 1 // +1 for newline
					}
					pos += x

					// If not already selecting, start selection
					if !m.isSelecting {
						m.selectionStart = pos
						m.isSelecting = true
					} else {
						// Update end position while dragging
						m.selectionEnd = pos
						m.updateViewportContent()
					}
				}
			}
		} else if msg.Type == tea.MouseMotion && m.isSelecting {
			// Update selection end while dragging
			fullText := strings.Join(m.output, "\n")
			lines := strings.Split(fullText, "\n")

			adjustedY := msg.Y + m.viewport.YOffset
			x := msg.X

			if adjustedY >= 0 && adjustedY < len(lines) {
				line := lines[adjustedY]
				if x >= 0 && x <= len(line) {
					// Calculate position in flattened string
					pos := 0
					for i := 0; i < adjustedY; i++ {
						pos += len(lines[i]) + 1
					}
					pos += x

					m.selectionEnd = pos
					m.updateViewportContent()
				}
			}
		}

		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case editorReadyMsg:
		// Editor model is ready, activate it
		m.editorModel = msg.model
		// Call the editor's Init to start its cursor blink timer
		return m, m.editorModel.Init()

	case commandResultMsg:
		// Command finished executing, add output
		m.commandExecuting = false
		wasAtBottom := m.viewport.AtBottom()
		m.output = append(m.output, msg.output...)
		m.updateViewportContent()
		if wasAtBottom {
			m.viewport.GotoBottom()
		}
		if msg.shouldQuit {
			return m, tea.Quit
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			// Context-aware: if text is selected, copy instead of exit
			if m.selectionStart >= 0 && m.selectionEnd >= 0 {
				selected := m.getSelectedText()
				if selected != "" {
					m.clearSelection()
					m.copyToMemFsClipboard(selected)
					m.updateViewportContent()
					m.viewport.GotoBottom()
					return m, nil
				}
			}
			// No selection - standard exit behavior
			return m, tea.Quit

		case tea.KeyCtrlV:
			// Paste from smart clipboard fallback
			clipboard := m.getClipboardToUse()
			if clipboard != "" {
				m.input = m.input[:m.cursor] + clipboard + m.input[m.cursor:]
				m.cursor += len(clipboard)
				m.viewport.GotoBottom()
			}
			return m, nil

		case tea.KeyEsc:
			// Clear any selection
			m.clearSelection()
			m.updateViewportContent()
			return m, nil

		case tea.KeyEnter:
			// Handle exit confirmation if pending
			if m.pendingExit {
				response := strings.ToLower(strings.TrimSpace(m.input))
				m.output = append(m.output, "(y/n) "+m.input)
				m.input = ""
				m.cursor = 0
				m.pendingExit = false
				if response == "y" || response == "yes" {
					m.output = append(m.output, "Goodbye!")
					m.updateViewportContent()
					m.viewport.GotoBottom()
					m.output = append(m.output, "")
					return m, tea.Quit
				} else {
					m.output = append(m.output, "Exit cancelled.")
					m.updateViewportContent()
					m.viewport.GotoBottom()
					m.output = append(m.output, "")
				}
				return m, nil
			}

			// Execute command
			if len(strings.TrimSpace(m.input)) > 0 {
				cmdToExecute := m.input
				m.history = append(m.history, m.input)

				// Add command echo and newline to output immediately
				m.output = append(m.output, promptStyle.Render(m.prompt+" ")+cmdToExecute)
				m.output = append(m.output, "")

				// Clear input and mark command as executing
				m.input = ""
				m.cursor = 0
				m.historyIndex = -1
				m.draft = ""
				m.commandExecuting = true

				// Update viewport to show the command echo and newline immediately
				m.updateViewportContent()
				m.viewport.GotoBottom()

				// Return a command that executes asynchronously
				return m, m.executeCommandAsync(cmdToExecute)
			} else {
				m.output = append(m.output, "")
				m.updateViewportContent()
			}

		case tea.KeyUp:
			if len(m.history) > 0 {
				if m.historyIndex == -1 {
					m.draft = m.input
					m.historyIndex = len(m.history) - 1
				} else if m.historyIndex > 0 {
					m.historyIndex--
				}
				if m.historyIndex >= 0 && m.historyIndex < len(m.history) {
					m.input = m.history[m.historyIndex]
					m.cursor = len(m.input)
				}
			}

		case tea.KeyDown:
			// Navigate history down
			if m.historyIndex >= 0 {
				if m.historyIndex < len(m.history)-1 {
					m.historyIndex++
					m.input = m.history[m.historyIndex]
					m.cursor = len(m.input)
				} else {
					m.historyIndex = -1
					m.input = m.draft
					m.cursor = len(m.input)
					m.draft = ""
				}
			}

		case tea.KeyBackspace:
			if m.cursor > 0 && len(m.input) > 0 {
				m.input = m.input[:m.cursor-1] + m.input[m.cursor:]
				m.cursor--
				// Scroll to bottom when user starts editing
				m.viewport.GotoBottom()
			}

		case tea.KeyLeft:
			if m.cursor > 0 {
				m.cursor--
			}

		case tea.KeyRight:
			if m.cursor < len(m.input) {
				m.cursor++
			}

		case tea.KeyHome:
			m.cursor = 0

		case tea.KeyEnd:
			m.cursor = len(m.input)

		case tea.KeyCtrlA:
			m.cursor = 0

		case tea.KeyCtrlE:
			m.cursor = len(m.input)

		case tea.KeyCtrlU:
			// Clear line
			m.input = ""
			m.cursor = 0
			// Scroll to bottom when user starts editing
			m.viewport.GotoBottom()

		case tea.KeyCtrlL:
			// Clear screen
			m.output = []string{}
			m.updateViewportContent()
			m.viewport.GotoTop()

		case tea.KeyPgUp, tea.KeyPgDown:
			// Forward scrolling to viewport
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd

		case tea.KeyTab:
			// Perform tab completion
			completed, options := m.tabComplete()
			if completed != "" {
				// Replace the path at cursor position with completed path
				m.input = completed
				m.cursor = len(m.input)
			} else if len(options) > 0 {
				// Multiple matches - show options to user
				m.output = append(m.output, "")
				m.output = append(m.output, strings.Join(options, "  "))
				m.updateViewportContent()
				m.viewport.GotoBottom()
			}

		default:
			// Insert character
			s := msg.String()
			if len(s) == 1 {
				m.input = m.input[:m.cursor] + s + m.input[m.cursor:]
				m.cursor++
				// Scroll to bottom when user starts typing
				m.viewport.GotoBottom()
			}
		}
	}

	return m, nil
}

// updateViewportContent updates the viewport with all output from the persistent buffer
func (m *ShellModel) updateViewportContent() {
	// If there's a selection, highlight it
	if m.selectionStart >= 0 && m.selectionEnd >= 0 {
		fullText := strings.Join(m.output, "\n")

		start := m.selectionStart
		end := m.selectionEnd
		if start > end {
			start, end = end, start
		}

		if start < 0 || end > len(fullText) {
			m.viewport.SetContent(strings.Join(m.output, "\n"))
			return
		}

		// Build highlighted version using reverse video (robust with existing ANSI codes)
		before := fullText[:start]
		selected := fullText[start:end]
		after := fullText[end:]

		highlightStyle := lipgloss.NewStyle().Reverse(true)
		highlighted := before + highlightStyle.Render(selected) + after
		m.viewport.SetContent(highlighted)
	} else {
		m.viewport.SetContent(strings.Join(m.output, "\n"))
	}
}

func (m *ShellModel) View() string {
	// If dirpicker is active, show dirpicker view instead
	if m.dirPickerMode && m.dirPicker != nil {
		return m.dirPicker.View()
	}

	// If editor is active, show editor view instead
	if m.editorModel != nil {
		return m.editorModel.View()
	}

	var sb strings.Builder

	// Render viewport content (persistent buffer with scrolling)
	sb.WriteString(m.viewport.View())

	// Display prompt and input only if not executing a command
	if !m.commandExecuting {
		sb.WriteString("\n")
		if m.pendingExit {
			sb.WriteString(promptStyle.Render("(y/n) "))
		} else {
			sb.WriteString(promptStyle.Render(m.prompt + " "))
		}

		// Render input with cursor
		if m.cursor < len(m.input) {
			before := m.input[:m.cursor]
			at := string(m.input[m.cursor])
			after := m.input[m.cursor+1:]

			if m.cursorVisible {
				cursorStyle := lipgloss.NewStyle().Reverse(true)
				sb.WriteString(before)
				sb.WriteString(cursorStyle.Render(at))
				sb.WriteString(after)
			} else {
				sb.WriteString(m.input)
			}
		} else {
			sb.WriteString(m.input)
			if m.cursorVisible {
				cursorStyle := lipgloss.NewStyle().Reverse(true)
				sb.WriteString(cursorStyle.Render(" "))
			}
		}
	}

	return sb.String()
}

// tabComplete attempts to complete the current input with file/directory paths
// Returns (completed input, options if multiple matches)
func (m *ShellModel) tabComplete() (string, []string) {
	// Parse the input to find the path to complete
	beforeCursor := m.input[:m.cursor]
	afterCursor := m.input[m.cursor:]

	// Find the last word (potential path) before cursor
	fields := strings.Fields(beforeCursor)
	if len(fields) == 0 {
		return "", nil
	}

	// Get the path to complete (last field)
	pathToComplete := fields[len(fields)-1]

	// Split into directory and prefix
	lastSlash := strings.LastIndex(pathToComplete, "/")
	var dir, prefix string
	if lastSlash == -1 {
		// No slash - completing in current directory
		dir = "."
		prefix = pathToComplete
	} else if lastSlash == 0 {
		// Root directory
		dir = "/"
		prefix = pathToComplete[1:]
	} else {
		// Some directory path
		dir = pathToComplete[:lastSlash]
		prefix = pathToComplete[lastSlash+1:]
	}

	// Find matching entries in the directory
	matches := m.findMatches(dir, prefix)

	if len(matches) == 0 {
		return "", nil
	} else if len(matches) == 1 {
		// Single match - complete it
		completed := matches[0]

		// Construct the new input
		beforePath := ""
		if len(fields) > 1 {
			beforePath = strings.Join(fields[:len(fields)-1], " ") + " "
		}

		// Build completed path
		var completedPath string
		if lastSlash == -1 {
			completedPath = completed
		} else {
			completedPath = pathToComplete[:lastSlash+1] + completed
		}

		newInput := beforePath + completedPath + afterCursor
		return newInput, nil
	} else {
		// Multiple matches - check for common prefix
		commonPrefix := findCommonPrefix(matches)
		if len(commonPrefix) > len(prefix) {
			// Complete to common prefix
			beforePath := ""
			if len(fields) > 1 {
				beforePath = strings.Join(fields[:len(fields)-1], " ") + " "
			}

			var completedPath string
			if lastSlash == -1 {
				completedPath = commonPrefix
			} else {
				completedPath = pathToComplete[:lastSlash+1] + commonPrefix
			}

			newInput := beforePath + completedPath + afterCursor
			return newInput, nil
		}
		// No further completion possible - return options
		return "", matches
	}
}

// findCommonPrefix finds the longest common prefix among strings
func findCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}

	prefix := strs[0]
	for i := 1; i < len(strs); i++ {
		// Find common prefix between current prefix and next string
		j := 0
		for j < len(prefix) && j < len(strs[i]) && prefix[j] == strs[i][j] {
			j++
		}
		prefix = prefix[:j]
		if len(prefix) == 0 {
			break
		}
	}
	return prefix
}

// findMatches finds all files/directories in dir that start with prefix
func (m *ShellModel) findMatches(dir, prefix string) []string {
	var matches []string

	// Read directory entries
	entries, err := m.memFs.ReadDir(dir)
	if err != nil {
		return matches
	}

	// Filter entries by prefix
	for _, entry := range entries {
		name := entry.Name()

		// Skip io directory (internal) and .clipboard file
		if name == "io" || name == ".clipboard" {
			continue
		}

		// Check if name starts with prefix
		if strings.HasPrefix(name, prefix) {
			// Add trailing slash for directories
			if entry.IsDir() {
				name += "/"
			}
			matches = append(matches, name)
		}
	}

	return matches
}

// executeCommandAsync returns a tea.Cmd that executes the command asynchronously
func (m *ShellModel) executeCommandAsync(cmd string) tea.Cmd {
	trimmedCmd := strings.TrimSpace(cmd)
	parts := strings.Fields(trimmedCmd)

	// Check if this is a tconfig command with -ofs flag (directory picker)
	if len(parts) > 0 && parts[0] == "tconfig" {
		// Check if -ofs flag is present
		hasOFS := false
		isDevQA := true // Default to dev if no -env flag specified

		for _, arg := range parts {
			if arg == "-ofs" {
				hasOFS = true
			}
			if strings.HasPrefix(arg, "-env=") {
				env := strings.TrimPrefix(arg, "-env=")
				if env == "prod" {
					isDevQA = false
				}
			}
		}

		if hasOFS {
			if !isDevQA {
				return func() tea.Msg {
					return commandResultMsg{
						output:     []string{errorStyle.Render("Error: -ofs flag is only available in dev/QA environments")},
						shouldQuit: false,
					}
				}
			}

			// Enter dirpicker mode
			return func() tea.Msg {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					homeDir = ""
				}
				return dirPickerStartMsg{
					startPath:  homeDir,
					pendingCmd: trimmedCmd,
				}
			}
		}
	}

	// Handle rosea command specially - it needs to suspend the tea program
	if len(parts) > 0 && parts[0] == "rosea" {
		if m.chatSenderChan == nil {
			return func() tea.Msg {
				return commandResultMsg{
					output:     []string{errorStyle.Render("Error: chat channel not available")},
					shouldQuit: false,
				}
			}
		}

		// Check if trying to open .clipboard file
		args := parts[1:]
		if len(args) > 0 && (args[0] == ".clipboard" || args[0] == "/.clipboard") {
			return func() tea.Msg {
				return commandResultMsg{
					output:     []string{errorStyle.Render("Error: cannot open .clipboard: No such file or directory")},
					shouldQuit: false,
				}
			}
		}

		// Return a command that requests editor model from rosea
		return func() tea.Msg {
			// Send message to trcshcmd/rosea to get editor model
			id := fmt.Sprintf("rosea-%d", time.Now().UnixNano())
			responseChan := make(chan tea.Model, 1)

			// Register hook for response
			GetChatMsgHooks().Set(id, func(msg *tccore.ChatMsg) bool {
				if msg.RoutingId != nil && *msg.RoutingId == id {
					if msg.HookResponse != nil {
						if editorModel, ok := msg.HookResponse.(tea.Model); ok {
							responseChan <- editorModel
						}
					}
					return true
				}
				return false
			})

			// Send request to launch editor
			pluginName := "trcsh"
			chatId := "rosea"
			roseaMsg := &tccore.ChatMsg{
				Name:         &pluginName,
				Query:        &[]string{"trcshcmd"},
				ChatId:       &chatId,
				RoutingId:    &id,
				HookResponse: args,
			}
			*m.chatSenderChan <- roseaMsg

			// Wait for editor model to be returned
			var editorModel tea.Model
			select {
			case editorModel = <-responseChan:
				GetChatMsgHooks().Remove(id)
			case <-time.After(5 * time.Second):
				GetChatMsgHooks().Remove(id)
				return commandResultMsg{
					output:     []string{errorStyle.Render("Error: timeout waiting for editor")},
					shouldQuit: false,
				}
			}

			return editorReadyMsg{model: editorModel}
		}
	}

	// For all other commands, use normal async execution
	return func() tea.Msg {
		output, shouldQuit := m.executeCommand(cmd)
		return commandResultMsg{
			output:     output,
			shouldQuit: shouldQuit,
		}
	}
}

// Message type sent when editor model is ready
type editorReadyMsg struct {
	model tea.Model
}

// executeCommand executes a command and returns the output lines and whether to quit
func (m *ShellModel) executeCommand(cmd string) ([]string, bool) {
	trimmedCmd := strings.TrimSpace(cmd)
	var output []string

	// Parse and execute command
	parts := strings.Fields(trimmedCmd)
	if len(parts) == 0 {
		return output, false
	}

	command := parts[0]
	args := parts[1:]

	switch command {
	case "exit", "quit":
		// If in elevated mode, exit just reverts to normal mode
		if m.elevatedMode {
			m.elevatedMode = false
			m.prompt = "$"
			output = append(output, "Exited elevated mode. Returned to normal access.")
			return output, false
		}
		// Otherwise, exit the shell
		output = append(output, "All uncommitted changes will be lost. Are you sure?")
		m.pendingExit = true
		// Don't add the normal empty line at the end for exit
		return output, false

	case "ls":
		// Determine which directory to list
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		if entries, err := m.memFs.ReadDir(dir); err == nil {
			// Filter out io directory and .clipboard file, and count visible entries
			visibleCount := 0
			for _, entry := range entries {
				name := entry.Name()
				// Skip io directory and .clipboard file
				if name == "io" || name == ".clipboard" {
					continue
				}
				visibleCount++
				if entry.IsDir() {
					name += "/"
				}
				output = append(output, name)
			}
			if visibleCount == 0 {
				output = append(output, ".")
			}
		} else {
			output = append(output, errorStyle.Render(fmt.Sprintf("Error reading directory: %v", err)))
		}

	case "tree":
		output = append(output, ".")
		treeOutput, dirCount, fileCount, err := m.printTree(".", "")
		if err != nil {
			output = append(output, errorStyle.Render(fmt.Sprintf("Error reading directory: %v", err)))
		} else {
			output = append(output, treeOutput...)
			output = append(output, "")
			output = append(output, fmt.Sprintf("%d directories, %d files", dirCount, fileCount))
		}

	case "rm":
		if m.chatSenderChan == nil {
			output = append(output, errorStyle.Render("Error: chat channel not available"))
			break
		}

		// Call trcshcmd for rm command with args
		response := CallTrcshCmd(m.chatSenderChan, "rm", args)
		if response != "" {
			// Split response by newlines and add each line
			lines := strings.Split(strings.TrimSpace(response), "\n")
			for _, line := range lines {
				output = append(output, line)
			}
		} else {
			output = append(output, "Files removed successfully")
		}

	case "cp":
		if m.chatSenderChan == nil {
			output = append(output, errorStyle.Render("Error: chat channel not available"))
			break
		}

		// Call trcshcmd for cp command with args
		response := CallTrcshCmd(m.chatSenderChan, "cp", args)
		if response != "" {
			// Split response by newlines and add each line
			lines := strings.Split(strings.TrimSpace(response), "\n")
			for _, line := range lines {
				output = append(output, line)
			}
		} else {
			output = append(output, "Files copied successfully")
		}

	case "mv":
		if m.chatSenderChan == nil {
			output = append(output, errorStyle.Render("Error: chat channel not available"))
			break
		}

		// Call trcshcmd for mv command with args
		response := CallTrcshCmd(m.chatSenderChan, "mv", args)
		if response != "" {
			// Split response by newlines and add each line
			lines := strings.Split(strings.TrimSpace(response), "\n")
			for _, line := range lines {
				output = append(output, line)
			}
		} else {
			output = append(output, "Files moved successfully")
		}

	case "cat":
		if m.chatSenderChan == nil {
			output = append(output, errorStyle.Render("Error: chat channel not available"))
			break
		}

		// Call trcshcmd for cat command with args
		response := CallTrcshCmd(m.chatSenderChan, "cat", args)
		if response != "" {
			// Split response by newlines and add each line
			lines := strings.Split(strings.TrimSpace(response), "\n")
			for _, line := range lines {
				output = append(output, line)
			}
		}

	case "mkdir":
		if m.chatSenderChan == nil {
			output = append(output, errorStyle.Render("Error: chat channel not available"))
			break
		}

		// Call trcshcmd for mkdir command with args
		response := CallTrcshCmd(m.chatSenderChan, "mkdir", args)
		if response != "" {
			// Split response by newlines and add each line
			lines := strings.Split(strings.TrimSpace(response), "\n")
			for _, line := range lines {
				output = append(output, line)
			}
		}

	case "tsub":
		if m.chatSenderChan == nil {
			output = append(output, errorStyle.Render("Error: chat channel not available"))
			break
		}

		// Call trcshcmd synchronously - let trcsub handle its own usage validation
		response := CallTrcshCmd(m.chatSenderChan, "tsub", args)
		if response != "" {
			// Split response by newlines and add each line
			lines := strings.Split(strings.TrimSpace(response), "\n")
			for _, line := range lines {
				output = append(output, line)
			}
		}

	// case "tpub":
	// 	// Only available in elevated mode
	// 	if !m.elevatedMode {
	// 		output = append(output, errorStyle.Render("Error: 'tpub' command requires elevated access"))
	// 		output = append(output, "Run 'su' to obtain elevated access")
	// 		break
	// 	}

	// 	if m.chatSenderChan == nil {
	// 		output = append(output, errorStyle.Render("Error: chat channel not available"))
	// 		break
	// 	}

	// 	// Call trcshcmd synchronously - let trcpub handle its own usage validation
	// 	response := callTrcshCmd(m.chatSenderChan, "tpub", args)
	// 	if response != "" {
	// 		// Split response by newlines and add each line
	// 		lines := strings.Split(strings.TrimSpace(response), "\n")
	// 		for _, line := range lines {
	// 			// Style authorization errors in red
	// 			if strings.Contains(line, "AUTHORIZATION ERROR") {
	// 				output = append(output, errorStyle.Render(line))
	// 			} else {
	// 				output = append(output, line)
	// 			}
	// 		}
	// 	}

	case "su":
		if m.chatSenderChan == nil {
			output = append(output, errorStyle.Render("Error: chat channel not available"))
			break
		}

		// Call trcshcmd to perform OAuth authentication for unrestricted access
		output = append(output, "Requesting elevated access...")
		output = append(output, "Waiting for authentication to complete...")
		response := callTrcshCmdWait(m.chatSenderChan, "su", args)
		if response != "" && strings.Contains(response, "success") {
			m.elevatedMode = true
			m.prompt = "#"
			// Split response by newlines and add each line
			lines := strings.Split(strings.TrimSpace(response), "\n")
			for _, line := range lines {
				output = append(output, line)
			}
			output = append(output, "")
			output = append(output, "Elevated mode activated. Additional commands available:")
			output = append(output, "  tinit    - Run trcinit commands (write access)")
			output = append(output, "  tx       - Run trcx commands (write access)")
			output = append(output, "Type 'exit' to return to normal mode.")
		} else if response == "" {
			// Timeout - return to prompt
			output = append(output, errorStyle.Render("Authentication timeout (15 seconds)"))
			output = append(output, errorStyle.Render("Try again or check your browser."))
		} else {
			// Authentication failed - return to prompt
			lines := strings.Split(strings.TrimSpace(response), "\n")
			for _, line := range lines {
				output = append(output, errorStyle.Render(line))
			}
		}

	case "tinit":
		// Only available in elevated mode
		if !m.elevatedMode {
			output = append(output, errorStyle.Render("Error: 'tinit' command requires elevated access"))
			output = append(output, "Run 'su' to obtain elevated access")
			break
		}

		if m.chatSenderChan == nil {
			output = append(output, errorStyle.Render("Error: chat channel not available"))
			break
		}

		// Call trcshcmd synchronously - let trcinit handle its own usage validation
		response := CallTrcshCmd(m.chatSenderChan, "tinit", args)
		if response != "" {
			// Split response by newlines and add each line
			lines := strings.Split(strings.TrimSpace(response), "\n")
			for _, line := range lines {
				// Style authorization errors in red
				if strings.Contains(line, "AUTHORIZATION ERROR") {
					output = append(output, errorStyle.Render(line))
				} else {
					output = append(output, line)
				}
			}

		}

	case "tx":
		if m.chatSenderChan == nil {
			output = append(output, errorStyle.Render("Error: chat channel not available"))
			break
		}

		// Call trcshcmd synchronously - let trcx handle its own usage validation
		response := CallTrcshCmd(m.chatSenderChan, "tx", args)
		if response != "" {
			// Split response by newlines and add each line
			lines := strings.Split(strings.TrimSpace(response), "\n")
			for _, line := range lines {
				// Style authorization errors in red
				if strings.Contains(line, "AUTHORIZATION ERROR") {
					output = append(output, errorStyle.Render(line))
				} else {
					output = append(output, line)
				}
			}
		}

	case "tconfig":
		if m.chatSenderChan == nil {
			output = append(output, errorStyle.Render("Error: chat channel not available"))
			break
		}

		// Call trcshcmd with args (and no -ofs handling here since it's now in executeCommandAsync)
		response := CallTrcshCmd(m.chatSenderChan, "tconfig", args)
		if response != "" {
			// Split response by newlines and add each line
			lines := strings.Split(strings.TrimSpace(response), "\n")
			for _, line := range lines {
				output = append(output, line)
			}
		}

	case "help":
		output = append(output, "Available commands:")
		output = append(output, "  help     - Show this help message")
		output = append(output, "  echo     - Echo arguments")
		output = append(output, "  ls       - List directory contents")
		output = append(output, "  tree     - Display directory tree structure")
		output = append(output, "  cat      - Display file contents")
		output = append(output, "  mkdir    - Create directories (use -p for parent directories)")
		output = append(output, "  rm       - Remove files or directories (use -r for recursive)")
		output = append(output, "  cp       - Copy files or directories (use -r for recursive)")
		output = append(output, "  mv       - Move/rename files or directories")
		output = append(output, "  clear    - Clear screen (or press Ctrl+L)")
		output = append(output, "  history  - Show command history")
		output = append(output, "  rosea    - Edit files with rosea editor")
		output = append(output, "  tsub     - Run trcsub commands")
		output = append(output, "  tconfig  - Run trcconfig commands")
		output = append(output, "  tx       - Run trcx commands")
		if !m.elevatedMode {
			output = append(output, "  su       - Obtain elevated access for write operations")
		} else {
			output = append(output, "  tinit    - Run trcinit commands (elevated mode only)")
			// output = append(output, "  tpub     - Run trcpub commands (elevated mode only)")
		}
		output = append(output, "  exit     - Exit shell (or press Ctrl+C when no text selected)")
		output = append(output, "")
		output = append(output, "Clipboard (isolated from system):")
		output = append(output, "  Ctrl+C   - Copy selected text to clipboard (if text selected), or exit shell")
		output = append(output, "  Ctrl+V   - Paste from internal clipboard (or system if newer)")
		output = append(output, "  Click+Drag - Select text by clicking and dragging")
		if m.elevatedMode {
			output = append(output, "")
			output = append(output, "Currently in elevated mode (#). Type 'exit' to return to normal mode.")
		}

	case "echo":
		output = append(output, strings.Join(args, " "))

	case "clear":
		// Clear command needs immediate effect, bypass async pattern
		m.output = []string{}
		m.updateViewportContent()
		m.viewport.GotoTop()
		return output, false

	case "history":
		if len(m.history) == 0 {
			output = append(output, "No command history")
		} else {
			for i, h := range m.history {
				output = append(output, fmt.Sprintf("%4d  %s", i+1, h))
			}
		}

	default:
		output = append(output, errorStyle.Render(fmt.Sprintf("Unknown command: %s", command)))
		output = append(output, "Type 'help' for available commands")
	}

	// Don't add empty line if waiting for exit confirmation
	if !m.pendingExit {
		output = append(output, "")
	}
	return output, false
}

// printTree recursively prints the directory tree structure
// Returns (output lines, dirCount, fileCount, error)
func (m *ShellModel) printTree(path string, prefix string) ([]string, int, int, error) {
	var treeOutput []string

	entries, err := m.memFs.ReadDir(path)
	if err != nil {
		return treeOutput, 0, 0, err
	}

	// Filter out io directory and .clipboard file
	filteredEntries := []os.FileInfo{}
	for _, entry := range entries {
		if entry.Name() != "io" && entry.Name() != ".clipboard" {
			filteredEntries = append(filteredEntries, entry)
		}
	}

	dirCount := 0
	fileCount := 0

	for i, entry := range filteredEntries {
		isLast := i == len(filteredEntries)-1
		var linePrefix, childPrefix string

		if isLast {
			linePrefix = prefix + "└── "
			childPrefix = prefix + "    "
		} else {
			linePrefix = prefix + "├── "
			childPrefix = prefix + "│   "
		}

		name := entry.Name()
		if entry.IsDir() {
			dirCount++
			treeOutput = append(treeOutput, linePrefix+name+"/")
			// Recursively print subdirectory
			subPath := path
			if path == "." {
				subPath = name
			} else {
				subPath = path + "/" + name
			}
			subOutput, subDirCount, subFileCount, err := m.printTree(subPath, childPrefix)
			if err != nil {
				treeOutput = append(treeOutput, childPrefix+errorStyle.Render(fmt.Sprintf("Error: %v", err)))
			} else {
				treeOutput = append(treeOutput, subOutput...)
				dirCount += subDirCount
				fileCount += subFileCount
			}
		} else {
			fileCount++
			treeOutput = append(treeOutput, linePrefix+name)
		}
	}

	return treeOutput, dirCount, fileCount, nil
}

var globalShellModel *ShellModel

// CallTrcshCmd sends a command to trcshcmd and waits for the response
func CallTrcshCmd(chatSenderChan *chan *tccore.ChatMsg, cmdType string, args []string) string {
	id := fmt.Sprintf("%s-%d", cmdType, time.Now().UnixNano())
	responseChan := make(chan string, 1)

	// Register hook for response
	GetChatMsgHooks().Set(id, func(msg *tccore.ChatMsg) bool {
		if msg.RoutingId != nil && *msg.RoutingId == id {
			if msg.Response != nil {
				go func() {
					responseChan <- *msg.Response
				}()
			} else {
				go func() {
					responseChan <- ""
				}()
			}
			return true
		}
		return false
	})

	// Send request
	pluginName := "trcsh"
	msg := &tccore.ChatMsg{
		Name:         &pluginName,
		Query:        &[]string{"trcshcmd"},
		ChatId:       &cmdType,
		RoutingId:    &id,
		HookResponse: args,
	}

	go func() {
		*chatSenderChan <- msg
	}()

	// Wait for response with timeout
	select {
	case response := <-responseChan:
		GetChatMsgHooks().Remove(id)
		return response
	case <-time.After(11 * time.Second):
		GetChatMsgHooks().Remove(id)
		return ""
	}
}

// callTrcshCmdWait sends a command to trcshcmd and waits for the response with a 15-second timeout
// Used for commands like 'su' that need to wait for user authentication
// Returns empty string on timeout
func callTrcshCmdWait(chatSenderChan *chan *tccore.ChatMsg, cmdType string, args []string) string {
	id := fmt.Sprintf("%s-%d", cmdType, time.Now().UnixNano())
	responseChan := make(chan string, 1)

	// Register hook for response
	GetChatMsgHooks().Set(id, func(msg *tccore.ChatMsg) bool {
		if msg.RoutingId != nil && *msg.RoutingId == id {
			if msg.Response != nil {
				go func() {
					responseChan <- *msg.Response
				}()
			} else {
				go func() {
					responseChan <- ""
				}()
			}
			return true
		}
		return false
	})

	// Send request
	pluginName := "trcsh"
	msg := &tccore.ChatMsg{
		Name:         &pluginName,
		Query:        &[]string{"trcshcmd"},
		ChatId:       &cmdType,
		RoutingId:    &id,
		HookResponse: args,
	}

	go func() {
		*chatSenderChan <- msg
	}()

	// Wait for response with 15-second timeout
	select {
	case response := <-responseChan:
		GetChatMsgHooks().Remove(id)
		return response
	case <-time.After(15 * time.Second):
		GetChatMsgHooks().Remove(id)
		return ""
	}
}

func RunShell(chatSenderChan *chan *tccore.ChatMsg, memFs ...trcshio.MemoryFileSystem) error {
	model := InitShell(chatSenderChan, memFs...)
	globalShellModel = model
	// Use alternate screen and enable mouse support - this ensures proper terminal restoration
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	globalShellModel = nil
	return err
}
