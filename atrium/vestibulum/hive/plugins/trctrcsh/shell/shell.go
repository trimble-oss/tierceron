package shell

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

var (
	promptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	outputStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

type ShellModel struct {
	width        int
	height       int
	prompt       string
	input        string
	cursor       int
	history      []string
	historyIndex int
	draft        string
	output       []string
	scrollOffset int
}

func InitShell() *ShellModel {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width = 80
		height = 24
	}

	return &ShellModel{
		width:        width,
		height:       height,
		prompt:       "$",
		input:        "",
		cursor:       0,
		history:      []string{},
		historyIndex: -1,
		draft:        "",
		output:       []string{"Welcome to trcsh interactive shell", "Type 'help' for available commands, 'exit' or Ctrl+C to quit", ""},
		scrollOffset: 0,
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

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEnter:
			// Execute command
			if len(strings.TrimSpace(m.input)) > 0 {
				m.executeCommand(m.input)
				m.history = append(m.history, m.input)
				m.input = ""
				m.cursor = 0
				m.historyIndex = -1
				m.draft = ""
			} else {
				m.output = append(m.output, "")
			}

		case tea.KeyUp:
			// Navigate history up
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

		default:
			// Insert character
			s := msg.String()
			if len(s) == 1 {
				m.input = m.input[:m.cursor] + s + m.input[m.cursor:]
				m.cursor++
			}
		}
	}

	// Auto-scroll to bottom if needed
	visibleLines := m.height - 3 // Reserve space for prompt
	if len(m.output) > visibleLines {
		m.scrollOffset = len(m.output) - visibleLines
	}

	return m, nil
}

func (m *ShellModel) View() string {
	var sb strings.Builder

	// Display output history
	visibleLines := m.height - 3
	startLine := m.scrollOffset
	endLine := startLine + visibleLines
	if endLine > len(m.output) {
		endLine = len(m.output)
	}

	for i := startLine; i < endLine; i++ {
		sb.WriteString(outputStyle.Render(m.output[i]))
		sb.WriteString("\n")
	}

	// Display prompt and input
	sb.WriteString("\n")
	sb.WriteString(promptStyle.Render(m.prompt + " "))

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

func (m *ShellModel) executeCommand(cmd string) {
	trimmedCmd := strings.TrimSpace(cmd)

	// Add command to output
	m.output = append(m.output, promptStyle.Render(m.prompt+" ")+cmd)

	// Parse and execute command
	parts := strings.Fields(trimmedCmd)
	if len(parts) == 0 {
		return
	}

	command := parts[0]
	args := parts[1:]

	switch command {
	case "exit", "quit":
		m.output = append(m.output, "Goodbye!")
		// Note: actual exit will be handled by Ctrl+C or the program can call tea.Quit

	case "help":
		m.output = append(m.output, "Available commands:")
		m.output = append(m.output, "  help     - Show this help message")
		m.output = append(m.output, "  echo     - Echo arguments")
		m.output = append(m.output, "  clear    - Clear screen (or press Ctrl+L)")
		m.output = append(m.output, "  history  - Show command history")
		m.output = append(m.output, "  exit     - Exit shell (or press Ctrl+C)")

	case "echo":
		m.output = append(m.output, strings.Join(args, " "))

	case "clear":
		m.output = []string{}

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

	m.output = append(m.output, "")
}

func RunShell() error {
	p := tea.NewProgram(InitShell())
	_, err := p.Run()
	return err
}
