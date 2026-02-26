package testr

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcrosea/hcore/flowutil"
	roseacore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcrosea/rosea/core"
	"golang.org/x/term"
)

// EditorCloseMsg signals that the editor should be closed
type EditorCloseMsg struct{}

// CloseEditor returns a command that sends EditorCloseMsg
func CloseEditor() tea.Cmd {
	return func() tea.Msg {
		return EditorCloseMsg{}
	}
}

// cursorBlinkMsg triggers cursor blink in editor
type cursorBlinkMsg struct{}

func cursorBlink() tea.Cmd {
	return tea.Tick(time.Millisecond*530, func(t time.Time) tea.Msg {
		return cursorBlinkMsg{}
	})
}

// readMemFsClipboard tries to read from trcsh's memory filesystem clipboard
func readMemFsClipboard() (string, time.Time) {
	_, memfs := roseacore.GetRoseaMemFs()
	if memfs == nil {
		return "", time.Time{}
	}

	file, err := memfs.Open("/.clipboard")
	if err != nil {
		return "", time.Time{}
	}
	defer file.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, file)
	if err != nil {
		return "", time.Time{}
	}

	return buf.String(), time.Now()
}

// getClipboardContent retrieves clipboard content, checking memFs first, then system
// Prefers clipboard with different content (user's latest action), otherwise prefers newer
// This allows rosea to be aware of trcsh's clipboard
func getClipboardContent() string {
	// First check trcsh's memory filesystem clipboard
	memfsContent, memfsTime := readMemFsClipboard()

	// Then try system clipboard
	var sysContent string
	var sysTime time.Time

	// Try WSL first (PowerShell Get-Clipboard)
	if content, err := tryWSLClipboard(); err == nil && content != "" {
		sysContent = content
	} else if content, err := tryWaylandClipboard(); err == nil && content != "" {
		// Try Wayland (wl-paste)
		sysContent = content
	} else if content, err := tryX11ClipboardXclip(); err == nil && content != "" {
		// Try X11 with xclip
		sysContent = content
	} else if content, err := tryX11ClipboardXsel(); err == nil && content != "" {
		// Try X11 with xsel
		sysContent = content
	}

	// Update system clipboard tracking only if content is new (using hash comparison)
	// Use hash instead of storing full content to avoid memory bloat
	if sysContent != "" {
		contentHash := hashContent(sysContent)
		if contentHash != lastSysClipHash {
			lastSysClipTime = time.Now()
			lastSysClipHash = contentHash
			sysTime = lastSysClipTime
		} else {
			sysTime = lastSysClipTime
		}
	} else {
		sysTime = lastSysClipTime
	}

	// Update memfs clipboard tracking only if content is new
	if memfsContent != "" && memfsContent != lastMemFsClipContent {
		lastMemFsClipTime = time.Now()
		lastMemFsClipContent = memfsContent
		memfsTime = lastMemFsClipTime
	} else {
		memfsTime = lastMemFsClipTime
	}

	// Return whichever is newer
	if memfsContent == "" {
		return sysContent
	}
	if sysContent == "" {
		return memfsContent
	}

	// Both have content
	// Use the one that was updated more recently
	if sysTime.After(memfsTime) {
		return sysContent
	}
	return memfsContent
}

// tryWSLClipboard tries to get clipboard from WSL using PowerShell
func tryWSLClipboard() (string, error) {
	cmd := exec.Command("powershell.exe", "-command", "Get-Clipboard")
	content, err := readClipboardWithSize(cmd)
	if err != nil {
		return "", err
	}
	// Remove Windows line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimRight(content, "\n")
	return content, nil
}

// tryWaylandClipboard tries to get clipboard from Wayland
func tryWaylandClipboard() (string, error) {
	cmd := exec.Command("wl-paste", "--no-newline")
	return readClipboardWithSize(cmd)
}

// tryX11ClipboardXclip tries to get clipboard from X11 using xclip
func tryX11ClipboardXclip() (string, error) {
	cmd := exec.Command("xclip", "-selection", "clipboard", "-o")
	return readClipboardWithSize(cmd)
}

// tryX11ClipboardXsel tries to get clipboard from X11 using xsel
func tryX11ClipboardXsel() (string, error) {
	cmd := exec.Command("xsel", "--clipboard", "--output")
	return readClipboardWithSize(cmd)
}

// Rosé Pine Moon styles
var (
	baseStyle = lipgloss.NewStyle().Background(lipgloss.Color("#000000")).Foreground(lipgloss.Color("#e0def4"))
	roseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#eb6f92"))
	pineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ccfd8"))
	foamStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00ff41")).
			Background(lipgloss.Color("#000000"))
	editedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ebbcba")).
			Background(lipgloss.Color("#000000"))
	goldStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#f6c177"))
	ctrlKeyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#00ff41")).Bold(true)
)

// Global tracking for memory filesystem clipboard changes (shared across instances)
var (
	lastMemFsClipContent string
	lastMemFsClipTime    time.Time
	lastSysClipHash      string // Hash of system clipboard (not full content)
	lastSysClipTime      time.Time
)

// Maximum clipboard size (10MB) - prevents OOM from malicious/accidental huge pastes
const maxClipboardSize = 10 * 1024 * 1024

// readClipboardWithSize reads clipboard output and truncates if over maxClipboardSize
func readClipboardWithSize(cmd *exec.Cmd) (string, error) {
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	content := out.String()
	if len(content) > maxClipboardSize {
		// Truncate to max size
		return content[:maxClipboardSize], nil
	}
	return content, nil
}

// hashContent returns the MD5 hash of content as a hex string
// Used for detecting clipboard changes without storing full content
func hashContent(content string) string {
	hash := md5.Sum([]byte(content))
	return fmt.Sprintf("%x", hash)
}

type RoseaEditorModel struct {
	title         string
	width         int      // terminal width
	lines         []string // Committed lines
	input         string   // Current input (multi-line)
	cursor        int      // Cursor position in input
	cursorVisible bool     // For blinking cursor
	historyIndex  int      // 0 = live input, 1 = last, 2 = second last, etc.
	draft         string   // Saved live input when entering history mode
	draftCursor   int

	// Authentication related fields
	showAuthPopup bool
	authInput     string
	authCursor    int
	authError     string
	popupMode     string // "token" or "confirm"
	editorStyle   lipgloss.Style

	scrollOffset int
	height       int
	roseaMode    bool // If true, save directly to memfs instead of trcdb

	// Write Out confirmation
	confirmingWrite bool // If true, showing confirmation prompt for Ctrl+O
	confirmCursor   bool // Cursor visibility for confirmation prompt

	// Modification tracking
	modified       bool // If true, buffer has unsaved changes
	confirmingExit bool // If true, showing exit confirmation for unsaved changes

	// Text selection for copy/paste
	selectionStart int  // Start position for text selection (-1 = no selection)
	selectionEnd   int  // End position for text selection
	isSelecting    bool // Flag to track if user is currently selecting/dragging

	// Multi-click tracking
	lastClickTime time.Time // Time of last click for multi-click detection
	lastClickX    int       // X position of last click
	lastClickY    int       // Y position of last click
	clickCount    int       // Click counter for multi-click detection
	hadMotion     bool      // Flag to track if mouse moved since click
}

func lines(b *[]byte) []string {
	var lines []string
	start := 0

	for i, c := range *b {
		if c == '\n' {
			end := i
			if end > start && (*b)[end-1] == '\r' {
				end--
			}
			lines = append(lines, string((*b)[start:end]))
			start = i + 1
		}
	}

	if start < len(*b) {
		end := len(*b)
		if end > start && (*b)[end-1] == '\r' {
			end--
		}
		lines = append(lines, string((*b)[start:end]))
	}

	return lines
}

// getSelectedText returns the currently selected text
func (m *RoseaEditorModel) getSelectedText() string {
	if m.selectionStart < 0 || m.selectionEnd < 0 {
		return ""
	}

	start := m.selectionStart
	end := m.selectionEnd
	if start > end {
		start, end = end, start
	}

	if start < 0 || end > len(m.input) {
		return ""
	}

	return m.input[start:end]
}

// clearSelection resets the selection
func (m *RoseaEditorModel) clearSelection() {
	m.selectionStart = -1
	m.selectionEnd = -1
}

// copyToMemFsClipboard stores content in memFs clipboard with retry logic
func (m *RoseaEditorModel) copyToMemFsClipboard(content string) {
	_, memfs := roseacore.GetRoseaMemFs()
	if memfs == nil {
		return
	}

	// Update global tracking with the new content
	lastMemFsClipContent = content
	lastMemFsClipTime = time.Now()

	// Retry up to 3 times to write to clipboard
	for attempt := 0; attempt < 3; attempt++ {
		// Only remove if file exists (skip on attempt 0 since it won't exist)
		if attempt > 0 {
			memfs.Remove("/.clipboard")
		}

		// Create new clipboard file and write content
		file, err := memfs.Create("/.clipboard")
		if err != nil {
			// Retry on error
			continue
		}

		_, err = file.Write([]byte(content))
		file.Close()

		if err == nil {
			// Success
			return
		}
		// Retry on write error
	}
}

func InitRoseaEditor(title string, data *[]byte) *RoseaEditorModel {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width = 80
		height = 24
	}

	// Initialize system clipboard hash at startup
	// This prevents treating stale pre-existing content as "newly changed"
	if sysContent, err := tryWSLClipboard(); err == nil && sysContent != "" {
		lastSysClipHash = hashContent(sysContent)
	} else if sysContent, err := tryWaylandClipboard(); err == nil && sysContent != "" {
		lastSysClipHash = hashContent(sysContent)
	} else if sysContent, err := tryX11ClipboardXclip(); err == nil && sysContent != "" {
		lastSysClipHash = hashContent(sysContent)
	} else if sysContent, err := tryX11ClipboardXsel(); err == nil && sysContent != "" {
		lastSysClipHash = hashContent(sysContent)
	}

	initialContent := strings.Join(lines(data), "\n")
	return &RoseaEditorModel{
		title:          title,
		width:          width,
		height:         height,
		lines:          []string{},
		input:          initialContent, // Initialize input with existing lines
		cursor:         0,
		cursorVisible:  true,
		historyIndex:   0,
		draft:          "",
		editorStyle:    baseStyle.Padding(1, 2).Width(width),
		roseaMode:      true, // Enable rosea mode for direct memfs save
		modified:       false,
		selectionStart: -1,
		selectionEnd:   -1,
	}
}

func (m *RoseaEditorModel) Init() tea.Cmd {
	return cursorBlink()
}

// isWordChar checks if a character is part of a word (alphanumeric or underscore)
func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// findWordBoundaries returns the start and end of the word at position pos
func findWordBoundaries(text string, pos int) (start, end int) {
	if pos < 0 || pos >= len(text) {
		return pos, pos
	}

	// If clicking on whitespace, select the whitespace
	if !isWordChar(text[pos]) {
		start = pos
		end = pos + 1
		return
	}

	// Find word start
	start = pos
	for start > 0 && isWordChar(text[start-1]) {
		start--
	}

	// Find word end
	end = pos + 1
	for end < len(text) && isWordChar(text[end]) {
		end++
	}

	return
}

// selectLine returns the start and end of the line containing pos
func (m *RoseaEditorModel) selectLine(text string, pos int) (start, end int) {
	if pos < 0 || pos > len(text) {
		return 0, 0
	}

	// Find line start
	start = pos
	for start > 0 && text[start-1] != '\n' {
		start--
	}

	// Find line end
	end = pos
	for end < len(text) && text[end] != '\n' {
		end++
	}

	return
}

func (m *RoseaEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case cursorBlinkMsg:
		m.cursorVisible = !m.cursorVisible
		if m.confirmingWrite {
			m.confirmCursor = !m.confirmCursor
		}
		return m, cursorBlink()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.MouseMsg:
		// Handle mouse events for selection with deferred multi-click detection
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			// Record click information for multi-click detection
			now := time.Now()
			const doubleClickThreshold = 300 * time.Millisecond
			const clickProximity = 5

			timeSinceLastClick := now.Sub(m.lastClickTime)
			proximityOK := (msg.X >= m.lastClickX-clickProximity && msg.X <= m.lastClickX+clickProximity) &&
				(msg.Y >= m.lastClickY-clickProximity && msg.Y <= m.lastClickY+clickProximity)

			// Increment click count if within threshold, proximity, and no motion since last click
			if timeSinceLastClick <= doubleClickThreshold && proximityOK && !m.hadMotion {
				m.clickCount++
			} else {
				m.clickCount = 1    // Reset for new click sequence
				m.hadMotion = false // Reset motion flag for new click sequence
			}

			// Record this click
			m.lastClickTime = now
			m.lastClickX = msg.X
			m.lastClickY = msg.Y

			// Get cursor position
			cursorPos := m.mousePosToCursorPos(msg.X, msg.Y)

			// Only process position change if not already selecting (first click)
			if !m.isSelecting {
				m.isSelecting = true
				if cursorPos >= 0 && cursorPos <= len(m.input) {
					m.selectionStart = cursorPos
					m.selectionEnd = cursorPos
				}
			} else {
				// Already selecting - this is a continued drag, update end position
				if cursorPos >= 0 && cursorPos <= len(m.input) {
					m.selectionEnd = cursorPos
				}
			}
		} else if msg.Action == tea.MouseActionMotion {
			// Mouse is moving while button held - mark that we had motion and update selection end
			m.hadMotion = true
			if m.isSelecting {
				cursorPos := m.mousePosToCursorPos(msg.X, msg.Y)
				if cursorPos >= 0 && cursorPos <= len(m.input) {
					m.selectionEnd = cursorPos
				}
			}
		} else if msg.Action == tea.MouseActionRelease {
			// Mouse button released - apply multi-click logic if no motion occurred
			if !m.hadMotion {
				// No motion - apply multi-click selection
				cursorPos := m.mousePosToCursorPos(msg.X, msg.Y)
				if cursorPos >= 0 && cursorPos <= len(m.input) {
					if m.clickCount == 2 {
						// Double-click: select word
						start, end := findWordBoundaries(m.input, cursorPos)
						m.selectionStart = start
						m.selectionEnd = end
					} else if m.clickCount >= 3 {
						// Triple-click: select entire line
						start, end := m.selectLine(m.input, cursorPos)
						m.selectionStart = start
						m.selectionEnd = end
					}
					// For single click, selection already set during MouseLeft
				}
			}
			// For motion-based selection, the end was already set during MouseMotion
			m.isSelecting = false
			// Don't reset hadMotion here - it will be reset on the next MouseLeft when a new sequence starts
		}
		return m, nil

	case tea.KeyMsg:
		// Handle exit confirmation state
		if m.confirmingExit {
			switch msg.Type {
			case tea.KeyRunes:
				key := msg.String()
				if key == "y" || key == "Y" {
					// Save and exit
					filename, memfs := roseacore.GetRoseaMemFs()
					if memfs != nil {
						// Prevent editing .clipboard file
						if filename == ".clipboard" || filename == "/.clipboard" {
							return m, CloseEditor()
						}
						memfs.Remove(filename)
						file, err := memfs.Create(filename)
						if err == nil {
							defer file.Close()
							io.WriteString(file, m.input)
						}
					}
					return m, CloseEditor()
				} else if key == "n" || key == "N" {
					// Exit without saving
					return m, CloseEditor()
				}
			case tea.KeyCtrlC:
				// Cancel and return to editor
				m.confirmingExit = false
				return m, nil
			default:
				// Ignore other keys
				return m, nil
			}
			return m, nil
		}

		// Handle write confirmation state
		if m.confirmingWrite {
			switch msg.Type {
			case tea.KeyCtrlC:
				// Cancel write
				m.confirmingWrite = false
				return m, nil
			case tea.KeyEnter:
				// Confirm write
				m.confirmingWrite = false
				filename, memfs := roseacore.GetRoseaMemFs()
				if memfs != nil {
					// Prevent editing .clipboard file
					if filename == ".clipboard" || filename == "/.clipboard" {
						return m, nil
					}
					memfs.Remove(filename)
					file, err := memfs.Create(filename)
					if err == nil {
						defer file.Close()
						io.WriteString(file, m.input)
						// Mark as saved
						m.modified = false

					}
				}
				return m, nil
			default:
				// Ignore other keys during confirmation
				return m, nil
			}
		}

		if m.showAuthPopup {
			switch m.popupMode {
			case "token":
				switch msg.Type {
				case tea.KeyEsc:
					m.input = m.draft
					//					m.cursor = 0
					m.cursor = m.draftCursor
					m.draft = ""
					m.showAuthPopup = false
					m.authInput = ""
					m.authCursor = 0
					m.authError = ""
				case tea.KeyEnter:
					if len(m.authInput) == 0 {
						m.authError = "Token cannot be empty"
					} else {
						m.input = m.draft
						m.cursor = m.draftCursor
						m.lines = append(m.lines, m.input)

						roseaSeedFile, roseaMemFs := roseacore.GetRoseaMemFs()
						roseaMemFs.Remove(roseaSeedFile)

						entrySeedFileRWC, err := roseaMemFs.Create(roseaSeedFile)
						if err != nil {
							// Pop up error?
							return m, nil
						}
						roseaEditR := strings.NewReader(m.input)
						_, err = io.Copy(entrySeedFileRWC, roseaEditR)
						if err != nil {
							// Pop up error?
							return m, nil
						}

						// Write current editor content to roseaMemFs
						chatResponseMsg := tccore.CallChatQueryChan(flowutil.GetChatMsgHookCtx(),
							"rosea", // From rainier
							&tccore.TrcdbExchange{
								Flows:     []string{flowcore.ArgosSociiFlow.TableName()},                                                                         // Flows
								Query:     fmt.Sprintf("SELECT * FROM %s.%s WHERE argosIdentitasNomen='%s'", "%s", flowcore.ArgosSociiFlow.TableName(), m.title), // Query
								Operation: "SELECT",                                                                                                              // query operation
								ExecTrcsh: "/edit/save.trc.tmpl",
								Request: tccore.TrcdbRequest{
									Rows: [][]any{
										{roseaMemFs},
										{m.authInput},
									},
								},
							},
							flowutil.GetChatSenderChan(),
						)
						if chatResponseMsg.TrcdbExchange != nil && len(chatResponseMsg.TrcdbExchange.Response.Rows) > 0 {
							// entrySeedFs := chatResponseMsg.TrcdbExchange.Request.Rows[0][0].(trcshio.MemoryFileSystem)
							// Chewbacca: If errors, maybe post an error message to popup?
						}
						m.historyIndex = 0
						// m.cursor = 0
						m.draft = ""
						m.showAuthPopup = false
						m.authError = ""
					}
					return m, nil
				case tea.KeyBackspace:
					if m.authCursor > 0 && len(m.authInput) > 0 {
						m.authInput = m.authInput[:m.authCursor-1] + m.authInput[m.authCursor:]
						m.authCursor--
					}
				case tea.KeyLeft:
					if m.authCursor > 0 {
						m.authCursor--
					}
				case tea.KeyRight:
					if m.authCursor < len(m.authInput) {
						m.authCursor++
					}
				default:
					s := msg.String()
					if len(s) > 0 && msg.Type != tea.KeySpace {
						s = roseacore.SanitizePaste(s)
						// Accept multi-character paste
						if m.showAuthPopup {
							m.authInput = m.authInput[:m.authCursor] + s + m.authInput[m.authCursor:]
							m.authCursor += len(s)
						} else {
							m.input = m.input[:m.cursor] + s + m.input[m.cursor:]
							m.cursor += len(s)
						}
					} else if msg.Type == tea.KeySpace {
						if m.showAuthPopup {
							m.authInput = m.authInput[:m.authCursor] + " " + m.authInput[m.authCursor:]
							m.authCursor++
						} else {
							m.input = m.input[:m.cursor] + " " + m.input[m.cursor:]
							m.cursor++
						}
					}
				}
				return m, nil
			case "confirm":
				switch msg.Type {
				case tea.KeyEnter:
					// Handle confirmation (proceed)
					m.showAuthPopup = false
					// ...do the action...
				case tea.KeyEsc:
					// Cancel
					m.showAuthPopup = false
				}
				return m, nil
			}
		}
		switch msg.Type {
		case tea.KeyCtrlC:
			// If text is selected, copy to clipboard instead of exiting
			selected := m.getSelectedText()
			if selected != "" {
				// Copy to system clipboard using xclip or xsel
				cmd := exec.Command("xclip", "-selection", "clipboard")
				cmd.Stdin = strings.NewReader(selected)
				if err := cmd.Run(); err != nil {
					cmd := exec.Command("xsel", "-b", "-i")
					cmd.Stdin = strings.NewReader(selected)
					cmd.Run()
				}
				// Always copy to memfs clipboard for paste functionality
				m.copyToMemFsClipboard(selected)
				m.clearSelection()
				return m, nil
			}
			// No selection - normal Ctrl+C behavior (exit)
			// Check for unsaved changes
			if m.modified {
				m.confirmingExit = true
				return m, nil
			}
			return m, CloseEditor()

		case tea.KeyCtrlX: // Exit
			if m.roseaMode {
				// Check for unsaved changes before exiting
				if m.modified {
					m.confirmingExit = true
					return m, nil
				}
				return m, CloseEditor()
			}
			return m, CloseEditor()

		case tea.KeyCtrlO: // Write Out (save)
			if m.roseaMode {
				// Show confirmation prompt
				m.confirmingWrite = true
				m.confirmCursor = true
			}
			return m, nil

		case tea.KeyCtrlV: // Paste from clipboard
			clipboardContent := getClipboardContent()
			if clipboardContent != "" {
				// Sanitize and insert at cursor
				clipboardContent = roseacore.SanitizePaste(clipboardContent)
				m.input = m.input[:m.cursor] + clipboardContent + m.input[m.cursor:]
				m.cursor += len(clipboardContent)
				m.modified = true
			}
			m.clearSelection()
			return m, nil

		case tea.KeyCtrlA: // Select all
			m.selectionStart = 0
			m.selectionEnd = len(m.input)
			return m, nil

		case tea.KeyEsc: // Clear selection on Escape
			m.clearSelection()
			return m, nil

		case tea.KeyCtrlS: // Submit on Ctrl+S (also saves)
			if m.roseaMode {
				// Rosea mode: save directly to memfs without auth popup
				filename, memfs := roseacore.GetRoseaMemFs()
				if memfs != nil {
					// Prevent editing .clipboard file
					if filename != ".clipboard" && filename != "/.clipboard" {
						memfs.Remove(filename)
						file, err := memfs.Create(filename)
						if err == nil {
							defer file.Close()
							io.WriteString(file, m.input)
							// Mark as saved
							m.modified = false

							// Return to shell after save
							return m, CloseEditor()
						}
					}
				}
				return m, nil
			} else {
				// Original trcdb mode: show auth popup
				m.draft = m.input
				m.draftCursor = m.cursor
				m.input = ""
				m.cursor = 0
				m.scrollOffset = 0
				m.showAuthPopup = true
				m.popupMode = "token"
				m.authInput = ""
				m.input = ""
				m.authCursor = 0
				m.authError = ""
			}
			return m, nil

		case tea.KeyEnter:
			// Insert newline at cursor
			m.input = m.input[:m.cursor] + "\n" + m.input[m.cursor:]
			m.cursor++
			m.modified = true
			m.clearSelection()
			return m, nil

		case tea.KeyBackspace:
			if m.cursor > 0 && len(m.input) > 0 {
				m.input = m.input[:m.cursor-1] + m.input[m.cursor:]
				m.cursor--
				m.modified = true
			}
			m.clearSelection()
			return m, nil

		case tea.KeyLeft:
			if m.cursor > 0 {
				m.cursor--
			}
			m.clearSelection()
			return m, nil

		case tea.KeyRight:
			if m.cursor < len(m.input) {
				m.cursor++
			}
			m.clearSelection()
			return m, nil

		case tea.KeyUp:
			visibleHeight := m.height - 4
			row, col := cursorRowCol(m.input, m.cursor)

			if row < m.scrollOffset {
				m.scrollOffset = row
			} else if row >= m.scrollOffset+visibleHeight {
				m.scrollOffset = row - visibleHeight + 1
			}
			if row > 0 {
				prevLineStart := nthLineStart(m.input, row-1)
				prevLineLen := lineLenAt(m.input, row-1)
				m.cursor = prevLineStart + min(col, prevLineLen)
			}
			return m, nil

		case tea.KeyDown:
			visibleHeight := m.height - 4
			row, col := cursorRowCol(m.input, m.cursor)

			if row < m.scrollOffset {
				m.scrollOffset = row
			} else if row >= m.scrollOffset+visibleHeight {
				m.scrollOffset = row - visibleHeight + 1
			}

			lineCount := strings.Count(m.input, "\n") + 1
			if row < lineCount-1 {
				nextLineStart := nthLineStart(m.input, row+1)
				nextLineLen := lineLenAt(m.input, row+1)
				m.cursor = nextLineStart + min(col, nextLineLen)
			}
			return m, nil

		default:
			s := msg.String()
			// Check for Ctrl+Shift+V (alternative paste shortcut)
			// This may come through as "ctrl+shift+v" or other variations depending on terminal
			if strings.ToLower(s) == "ctrl+shift+v" || (msg.Type == tea.KeyRunes && len(msg.Runes) > 0 && msg.Runes[0] == 22) {
				clipboardContent := getClipboardContent()
				if clipboardContent != "" {
					clipboardContent = roseacore.SanitizePaste(clipboardContent)
					m.input = m.input[:m.cursor] + clipboardContent + m.input[m.cursor:]
					m.cursor += len(clipboardContent)
					m.modified = true
				}
				return m, nil
			}
			if len(s) > 0 && msg.Type != tea.KeySpace {
				s = roseacore.SanitizePaste(s)
				// Accept multi-character paste
				if m.showAuthPopup {
					m.authInput = m.authInput[:m.authCursor] + s + m.authInput[m.authCursor:]
					m.authCursor += len(s)
				} else {
					m.input = m.input[:m.cursor] + s + m.input[m.cursor:]
					m.cursor += len(s)
					m.modified = true
					m.clearSelection()
				}
			} else if msg.Type == tea.KeySpace {
				if m.showAuthPopup {
					m.authInput = m.authInput[:m.authCursor] + " " + m.authInput[m.authCursor:]
					m.authCursor++
				} else {
					m.input = m.input[:m.cursor] + " " + m.input[m.cursor:]
					m.cursor++
					m.modified = true
					m.clearSelection()
				}
			}
		}
	}

	return m, nil
}

// Helper functions for multi-line cursor movement
func cursorRowCol(s string, cursor int) (row, col int) {
	row = strings.Count(s[:cursor], "\n")
	lastNL := strings.LastIndex(s[:cursor], "\n")
	if lastNL == -1 {
		col = cursor
	} else {
		col = cursor - lastNL - 1
	}
	return
}

func nthLineStart(s string, n int) int {
	if n == 0 {
		return 0
	}
	i := 0
	for l := 0; l < n; l++ {
		j := strings.IndexByte(s[i:], '\n')
		if j == -1 {
			return len(s)
		}
		i += j + 1
	}
	return i
}

func lineLenAt(s string, n int) int {
	start := nthLineStart(s, n)
	end := strings.IndexByte(s[start:], '\n')
	if end == -1 {
		return len(s) - start
	}
	return end
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// mousePosToCursorPos converts mouse coordinates to a position in the input text
func (m *RoseaEditorModel) mousePosToCursorPos(mouseX, mouseY int) int {
	// Account for header (1 line) and committed lines
	committedLinesCount := len(m.lines)
	headerAndCommittedHeight := 1 + committedLinesCount // 1 for "Roséa Multi-line Editor"

	// Check if click is in the input area
	if mouseY < headerAndCommittedHeight {
		return -1 // Click is in the header or committed lines area
	}

	// Calculate the row in the input area (0-indexed)
	inputRow := mouseY - headerAndCommittedHeight

	// Split input into lines
	inputLines := strings.Split(m.input, "\n")
	visibleHeight := m.height - 4 // Same calculation as View()
	startLine := m.scrollOffset
	endLine := min(len(inputLines), startLine+visibleHeight)

	// Check if click is within visible input area
	if inputRow >= endLine-startLine {
		return -1
	}

	// The actual line index
	actualLineIdx := startLine + inputRow

	// Calculate the absolute position at the start of this line
	lineStartPos := 0
	for i := 0; i < actualLineIdx; i++ {
		lineStartPos += len(inputLines[i]) + 1 // +1 for newline
	}

	// Get the line content
	if actualLineIdx >= len(inputLines) {
		return -1
	}
	line := inputLines[actualLineIdx]

	// The column is the mouse X position, but we need to account for the left margin
	// In the View() function, content starts at column 0
	col := mouseX
	if col > len(line) {
		col = len(line)
	}
	if col < 0 {
		col = 0
	}

	return lineStartPos + col
}

// renderLineWithSelection renders a line with both cursor and selection highlighting
func renderLineWithSelection(line string, cursorCol int, cursorVisible bool, lineStartPos int, lineEndPos int, selStart int, selEnd int) string {
	var result strings.Builder
	selectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#96c7a2"))
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#c4a7e7"))

	for i := 0; i < len(line); i++ {
		posInInput := lineStartPos + i
		isSelected := selStart >= 0 && posInInput >= selStart && posInInput < selEnd
		isCursor := i == cursorCol
		char := string(line[i])

		if isSelected {
			result.WriteString(selectionStyle.Render(char))
		} else if isCursor && cursorVisible {
			result.WriteString(cursorStyle.Render(char))
		} else {
			result.WriteString(foamStyle.Render(char))
		}
	}

	// Handle cursor at end of line
	if cursorCol == len(line) && cursorVisible {
		result.WriteString(cursorStyle.Render(" "))
	}

	return result.String()
}

// renderLineWithOnlySelection renders a line with just selection highlighting (no cursor)
func renderLineWithOnlySelection(line string, lineStartPos int, lineEndPos int, selStart int, selEnd int) string {
	var result strings.Builder
	selectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#96c7a2"))

	for i := 0; i < len(line); i++ {
		posInInput := lineStartPos + i
		isSelected := selStart >= 0 && posInInput >= selStart && posInInput < selEnd
		char := string(line[i])

		if isSelected {
			result.WriteString(selectionStyle.Render(char))
		} else {
			result.WriteString(foamStyle.Render(char))
		}
	}

	return result.String()
}

func (m *RoseaEditorModel) View() string {
	var b strings.Builder

	b.WriteString(roseStyle.Render("Roséa Multi-line Editor"))
	b.WriteString("\n")

	for _, line := range m.lines {
		b.WriteString(pineStyle.Render(line) + "\n")
	}

	// Render input with cursor and selection
	lines := strings.Split(m.input, "\n")
	visibleHeight := m.height - 4
	start := m.scrollOffset
	end := min(len(lines), start+visibleHeight)

	row, col := cursorRowCol(m.input, m.cursor)

	// Calculate selection range (if any)
	var selStart, selEnd int
	if m.selectionStart >= 0 && m.selectionEnd >= 0 {
		selStart = m.selectionStart
		selEnd = m.selectionEnd
		if selStart > selEnd {
			selStart, selEnd = selEnd, selStart
		}
	} else {
		selStart, selEnd = -1, -1
	}

	for i := start; i < end; i++ {
		line := lines[i]
		if i > start {
			b.WriteString("\n")
		}

		// Calculate the absolute position of the start and end of this line in the input
		lineStartPos := 0
		for j := 0; j < i; j++ {
			lineStartPos += len(lines[j]) + 1 // +1 for newline
		}
		lineEndPos := lineStartPos + len(line)

		if i == row {
			// Current line with cursor and optional selection
			b.WriteString(renderLineWithSelection(line, col, m.cursorVisible, lineStartPos, lineEndPos, selStart, selEnd))
		} else {
			// Other lines with optional selection
			b.WriteString(renderLineWithOnlySelection(line, lineStartPos, lineEndPos, selStart, selEnd))
		}
	}

	if m.showAuthPopup {
		var popupContent string
		switch m.popupMode {
		case "token":
			cursorChar := " "
			if m.authCursor < len(m.authInput) {
				cursorChar = string(m.authInput[m.authCursor])
			}
			cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#f6c177"))
			cursorBlock := ""
			if m.cursorVisible {
				cursorBlock = cursorStyle.Render(cursorChar)
			} else {
				cursorBlock = cursorChar
			}

			popupContent = "Enter authentication token:\n\n" +
				m.authInput[:m.authCursor] + cursorBlock + m.authInput[min(m.authCursor+1, len(m.authInput)):] +
				"\n\n" + m.authError + "\n\n[Enter=Submit, Esc=Cancel]"
		case "confirm":
			popupContent = "Are you sure you want to proceed?\n\n[Enter=Yes, Esc=Cancel]"
		}
		popup := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			Width(40).
			Align(lipgloss.Center).
			Render(popupContent)
		// Overlay the popup (simple version)
		b.WriteString("\n\n" + popup)
	}

	// Add rosea-style control bar or confirmation prompt at the bottom
	if m.roseaMode {
		b.WriteString("\n\n")
		if m.confirmingExit {
			// Show exit confirmation prompt with highlighted text
			promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff41")).Bold(true)
			promptText := promptStyle.Render("Save modified buffer?") + "  "
			optionsText := ctrlKeyStyle.Render(" Y ") + " Yes  " +
				ctrlKeyStyle.Render(" N ") + " No  " +
				ctrlKeyStyle.Render("^C") + " Cancel"
			b.WriteString(promptText + optionsText)
		} else if m.confirmingWrite {
			// Show write confirmation, replacing the control bar
			filename, _ := roseacore.GetRoseaMemFs()

			// Create cursor block for the filename
			cursorChar := " "
			if m.confirmCursor {
				cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#c4a7e7"))
				cursorChar = cursorStyle.Render(" ")
			}

			// Render confirmation in place of control bar
			filenameLine := "File Name to Write: " + filename + cursorChar
			controlLine := ctrlKeyStyle.Render("^C") + " Cancel"

			b.WriteString(filenameLine + "\n" + controlLine)
		} else {
			// Show normal control bar with highlighted control keys
			controlText := ctrlKeyStyle.Render("^X") + " Exit    " +
				ctrlKeyStyle.Render("^O") + " Write Out    " +
				ctrlKeyStyle.Render("^V") + " Paste"
			b.WriteString(controlText)
		}
	}

	// Add status line with help text
	b.WriteString("\n")
	helpText := "Ctrl+A: Select all | Ctrl+C: Copy | Ctrl+V: Paste | Ctrl+S: Save | Ctrl+X: Exit"
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#908caa"))
	b.WriteString(helpStyle.Render(helpText))

	return b.String()
}

// func main() {
// 	if err := tea.NewProgram(initialModel(nil)).Start(); err != nil {
// 		fmt.Fprintln(os.Stderr, "Error:", err)
// 		os.Exit(1)
// 	}
// }
