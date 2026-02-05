package shell

import (
	"fmt"
	"os"
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
)

var (
	promptStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	outputStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	chatMsgHooks = cmap.New[tccore.ChatHookFunc]()
)

// GetChatMsgHooks returns the chat message hooks map
func GetChatMsgHooks() *cmap.ConcurrentMap[string, tccore.ChatHookFunc] {
	return &chatMsgHooks
}

type ShellModel struct {
	width          int
	height         int
	prompt         string
	input          string
	cursor         int
	history        []string
	historyIndex   int
	draft          string
	output         []string       // Persistent buffer - holds ALL output
	viewport       viewport.Model // Viewport handles scrolling
	memFs          trcshio.MemoryFileSystem
	chatSenderChan *chan *tccore.ChatMsg
	pendingExit    bool
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
	vp.SetContent(strings.Join(initialOutput, "\\n"))

	return &ShellModel{
		width:          width,
		height:         height,
		prompt:         "$",
		input:          "",
		cursor:         0,
		history:        []string{},
		historyIndex:   -1,
		draft:          "",
		output:         initialOutput,
		viewport:       vp,
		memFs:          memFileSystem,
		chatSenderChan: chatSenderChan,
		pendingExit:    false,
	}
}

func (m *ShellModel) Init() tea.Cmd {
	return nil
}

func (m *ShellModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update viewport size (reserve 3 lines for prompt area)
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 3

	case tea.MouseMsg:
		// Forward mouse events to viewport for scrolling
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

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
				// Check if we're at bottom before executing (for auto-scroll decision)
				wasAtBottom := m.viewport.AtBottom()

				shouldQuit := m.executeCommand(m.input)
				m.history = append(m.history, m.input)
				m.input = ""
				m.cursor = 0
				m.historyIndex = -1
				m.draft = ""

				// Update viewport with new output
				m.updateViewportContent()

				// Auto-scroll to bottom only if we were already at bottom
				if wasAtBottom {
					m.viewport.GotoBottom()
				}

				if shouldQuit {
					return m, tea.Quit
				}
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

		case tea.KeyCtrlU:
			// Clear line
			m.input = ""
			m.cursor = 0

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

		default:
			// Insert character
			s := msg.String()
			if len(s) == 1 {
				m.input = m.input[:m.cursor] + s + m.input[m.cursor:]
				m.cursor++
			}
		}
	}

	return m, nil
}

// updateViewportContent updates the viewport with all output from the persistent buffer
func (m *ShellModel) updateViewportContent() {
	m.viewport.SetContent(strings.Join(m.output, "\n"))
}

func (m *ShellModel) View() string {
	var sb strings.Builder

	// Render viewport content (persistent buffer with scrolling)
	sb.WriteString(m.viewport.View())

	// Display prompt and input
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

		cursorStyle := lipgloss.NewStyle().Reverse(true)
		sb.WriteString(before)
		sb.WriteString(cursorStyle.Render(at))
		sb.WriteString(after)
	} else {
		sb.WriteString(m.input)
		cursorStyle := lipgloss.NewStyle().Reverse(true)
		sb.WriteString(cursorStyle.Render(" "))
	}

	return sb.String()
}

func (m *ShellModel) executeCommand(cmd string) bool {
	trimmedCmd := strings.TrimSpace(cmd)

	// Add command to output
	m.output = append(m.output, promptStyle.Render(m.prompt+" ")+cmd)

	// Parse and execute command
	parts := strings.Fields(trimmedCmd)
	if len(parts) == 0 {
		return false
	}

	command := parts[0]
	args := parts[1:]

	switch command {
	case "exit", "quit":
		m.output = append(m.output, "All uncommitted changes will be lost. Are you sure?")
		m.pendingExit = true
		// Don't add the normal empty line at the end for exit
		return false

	case "ls":
		// Determine which directory to list
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		if entries, err := m.memFs.ReadDir(dir); err == nil {
			if len(entries) == 0 {
				m.output = append(m.output, "(empty directory)")
			} else {
				for _, entry := range entries {
					name := entry.Name()
					// Skip io directory
					if name == "io" {
						continue
					}
					if entry.IsDir() {
						name += "/"
					}
					m.output = append(m.output, name)
				}
			}
		} else {
			m.output = append(m.output, errorStyle.Render(fmt.Sprintf("Error reading directory: %v", err)))
		}

	case "tree":
		m.output = append(m.output, ".")
		dirCount, fileCount, err := m.printTree(".", "")
		if err != nil {
			m.output = append(m.output, errorStyle.Render(fmt.Sprintf("Error reading directory: %v", err)))
		} else {
			m.output = append(m.output, "")
			m.output = append(m.output, fmt.Sprintf("%d directories, %d files", dirCount, fileCount))
		}

	case "tsub":
		if m.chatSenderChan == nil {
			m.output = append(m.output, errorStyle.Render("Error: chat channel not available"))
			break
		}

		// Call trcshcmd synchronously - let trcsub handle its own usage validation
		response := callTrcshCmd(m.chatSenderChan, "trcsub", args)
		if response != "" {
			// Split response by newlines and add each line
			lines := strings.Split(strings.TrimSpace(response), "\n")
			for _, line := range lines {
				m.output = append(m.output, line)
			}
		} else {
			m.output = append(m.output, errorStyle.Render("Error: no response from command"))
		}

	case "tpub":
		if m.chatSenderChan == nil {
			m.output = append(m.output, errorStyle.Render("Error: chat channel not available"))
			break
		}

		// Call trcshcmd synchronously - let trcpub handle its own usage validation
		response := callTrcshCmd(m.chatSenderChan, "trcpub", args)
		if response != "" {
			// Split response by newlines and add each line
			lines := strings.Split(strings.TrimSpace(response), "\n")
			for _, line := range lines {
				m.output = append(m.output, line)
			}
		} else {
			m.output = append(m.output, errorStyle.Render("Error: no response from command"))
		}

	case "tx":
		if m.chatSenderChan == nil {
			m.output = append(m.output, errorStyle.Render("Error: chat channel not available"))
			break
		}

		// Call trcshcmd synchronously - let trcx handle its own usage validation
		response := callTrcshCmd(m.chatSenderChan, "trcx", args)
		if response != "" {
			// Split response by newlines and add each line
			lines := strings.Split(strings.TrimSpace(response), "\n")
			for _, line := range lines {
				m.output = append(m.output, line)
			}
		} else {
			m.output = append(m.output, errorStyle.Render("Error: no response from command"))
		}

	case "tconfig":
		if m.chatSenderChan == nil {
			m.output = append(m.output, errorStyle.Render("Error: chat channel not available"))
			break
		}

		// Call trcshcmd synchronously - let trcconfig handle its own usage validation
		response := callTrcshCmd(m.chatSenderChan, "trcconfig", args)
		if response != "" {
			// Split response by newlines and add each line
			lines := strings.Split(strings.TrimSpace(response), "\n")
			for _, line := range lines {
				m.output = append(m.output, line)
			}
		} else {
			m.output = append(m.output, errorStyle.Render("Error: no response from command"))
		}

	case "help":
		m.output = append(m.output, "Available commands:")
		m.output = append(m.output, "  help     - Show this help message")
		m.output = append(m.output, "  echo     - Echo arguments")
		m.output = append(m.output, "  ls       - List directory contents")
		m.output = append(m.output, "  tree     - Display directory tree structure")
		m.output = append(m.output, "  clear    - Clear screen (or press Ctrl+L)")
		m.output = append(m.output, "  history  - Show command history")
		m.output = append(m.output, "  tsub     - Run trcsub commands")
		m.output = append(m.output, "  tpub     - Run trcpub commands")
		m.output = append(m.output, "  tx       - Run trcx commands")
		m.output = append(m.output, "  tconfig  - Run trcconfig commands")
		m.output = append(m.output, "  exit     - Exit shell (or press Ctrl+C)")

	case "echo":
		m.output = append(m.output, strings.Join(args, " "))

	case "clear":
		m.output = []string{}
		m.updateViewportContent()
		m.viewport.GotoTop()

	case "history":
		if len(m.history) == 0 {
			m.output = append(m.output, "No command history")
		} else {
			for i, h := range m.history {
				m.output = append(m.output, fmt.Sprintf("%4d  %s", i+1, h))
			}
		}

	default:
		m.output = append(m.output, errorStyle.Render(fmt.Sprintf("Unknown command: %s", command)))
		m.output = append(m.output, "Type 'help' for available commands")
	}

	// Don't add empty line if waiting for exit confirmation
	if !m.pendingExit {
		m.output = append(m.output, "")
	}
	return false
}

// printTree recursively prints the directory tree structure
// Returns (dirCount, fileCount, error)
func (m *ShellModel) printTree(path string, prefix string) (int, int, error) {
	entries, err := m.memFs.ReadDir(path)
	if err != nil {
		return 0, 0, err
	}

	// Filter out io directory
	filteredEntries := []os.FileInfo{}
	for _, entry := range entries {
		if entry.Name() != "io" {
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
			m.output = append(m.output, linePrefix+name+"/")
			// Recursively print subdirectory
			subPath := path
			if path == "." {
				subPath = name
			} else {
				subPath = path + "/" + name
			}
			subDirCount, subFileCount, err := m.printTree(subPath, childPrefix)
			if err != nil {
				m.output = append(m.output, childPrefix+errorStyle.Render(fmt.Sprintf("Error: %v", err)))
			} else {
				dirCount += subDirCount
				fileCount += subFileCount
			}
		} else {
			fileCount++
			m.output = append(m.output, linePrefix+name)
		}
	}

	return dirCount, fileCount, nil
}

var globalShellModel *ShellModel

// callTrcshCmd sends a command to trcshcmd and waits for the response
func callTrcshCmd(chatSenderChan *chan *tccore.ChatMsg, cmdType string, args []string) string {
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

func RunShell(chatSenderChan *chan *tccore.ChatMsg, memFs ...trcshio.MemoryFileSystem) error {
	model := InitShell(chatSenderChan, memFs...)
	globalShellModel = model
	// Enable mouse support for wheel scrolling
	p := tea.NewProgram(model, tea.WithMouseCellMotion())
	_, err := p.Run()
	globalShellModel = nil
	return err
}
