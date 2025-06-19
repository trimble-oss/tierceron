package testr

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	roseacore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcrosea/rosea/core"
	"golang.org/x/term"
)

// Rosé Pine Moon styles
var (
	baseStyle   = lipgloss.NewStyle().Background(lipgloss.Color("#232136")).Foreground(lipgloss.Color("#e0def4"))
	roseStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#eb6f92"))
	pineStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ccfd8"))
	foamStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#c4a7e7"))
	goldStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#f6c177"))
	editorStyle lipgloss.Style
)

type RoseaEditorModel struct {
	width        int      // terminal width
	lines        []string // Committed lines
	input        string   // Current input (multi-line)
	cursor       int      // Cursor position in input
	historyIndex int      // 0 = live input, 1 = last, 2 = second last, etc.
	draft        string   // Saved live input when entering history mode
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

func InitRoseaEditor(data *[]byte) RoseaEditorModel {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width = 80
	}
	editorStyleCopy := baseStyle
	editorStyle = editorStyleCopy.Padding(1, 2).Width(width)

	return RoseaEditorModel{
		width:        width,
		lines:        lines(data),
		input:        "",
		cursor:       0,
		historyIndex: 0,
		draft:        "",
	}
}

func (m RoseaEditorModel) Init() tea.Cmd {
	return nil
}

func (m RoseaEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			return roseacore.GetRoseaNavigationCtx(), nil

		case tea.KeyCtrlS: // Submit on Ctrl+S
			m.lines = append(m.lines, m.input)
			m.input = ""
			m.cursor = 0
			m.historyIndex = 0
			m.draft = ""

			return m, nil

		case tea.KeyEnter:
			// Insert newline at cursor
			m.input = m.input[:m.cursor] + "\n" + m.input[m.cursor:]
			m.cursor++
			return m, nil

		case tea.KeyBackspace:
			if m.cursor > 0 && len(m.input) > 0 {
				m.input = m.input[:m.cursor-1] + m.input[m.cursor:]
				m.cursor--
			}
			return m, nil

		case tea.KeyLeft:
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case tea.KeyRight:
			if m.cursor < len(m.input) {
				m.cursor++
			}
			return m, nil

		case tea.KeyUp:
			row, col := cursorRowCol(m.input, m.cursor)
			if row > 0 {
				prevLineStart := nthLineStart(m.input, row-1)
				prevLineLen := lineLenAt(m.input, row-1)
				m.cursor = prevLineStart + min(col, prevLineLen)
			}
			return m, nil

		case tea.KeyDown:
			row, col := cursorRowCol(m.input, m.cursor)
			lineCount := strings.Count(m.input, "\n") + 1
			if row < lineCount-1 {
				nextLineStart := nthLineStart(m.input, row+1)
				nextLineLen := lineLenAt(m.input, row+1)
				m.cursor = nextLineStart + min(col, nextLineLen)
			}
			return m, nil

		default:
			// Only insert printable characters
			if len(msg.String()) == 1 && msg.Type != tea.KeySpace {
				m.input = m.input[:m.cursor] + msg.String() + m.input[m.cursor:]
				m.cursor++
			} else if msg.Type == tea.KeySpace {
				m.input = m.input[:m.cursor] + " " + m.input[m.cursor:]
				m.cursor++
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

func (m RoseaEditorModel) View() string {
	var b strings.Builder

	b.WriteString(roseStyle.Render("Roséa Multi-line Editor — Ctrl+S to save, ESC to navigate"))
	b.WriteString("\n")

	for _, line := range m.lines {
		b.WriteString(pineStyle.Render(line) + "\n")
	}

	// Render input with cursor
	lines := strings.Split(m.input, "\n")
	row, col := cursorRowCol(m.input, m.cursor)
	for i, line := range lines {
		if i > 0 {
			b.WriteString("\n")
		}
		if i == row {
			left := foamStyle.Render(line[:min(col, len(line))])
			right := foamStyle.Render(line[min(col, len(line)):])
			cursor := goldStyle.Render("|")
			b.WriteString(left + cursor + right)
		} else {
			b.WriteString(foamStyle.Render(line))
		}
	}

	return editorStyle.Width(m.width).Render(b.String())
}

// func main() {
// 	if err := tea.NewProgram(initialModel(nil)).Start(); err != nil {
// 		fmt.Println("Error:", err)
// 		os.Exit(1)
// 	}
// }
