package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Rosé Pine Moon styles
var (
	baseStyle   = lipgloss.NewStyle().Background(lipgloss.Color("#232136")).Foreground(lipgloss.Color("#e0def4"))
	roseStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#eb6f92"))
	pineStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ccfd8"))
	foamStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#c4a7e7"))
	goldStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#f6c177"))
	editorStyle = baseStyle.Copy().Padding(1, 2).Width(60)
)

type model struct {
	lines        []string // Committed lines
	input        string   // Current input (multi-line)
	cursor       int      // Cursor position in input
	historyIndex int      // 0 = live input, 1 = last, 2 = second last, etc.
	draft        string   // Saved live input when entering history mode
}

func initialModel() model {
	return model{
		lines:        []string{},
		input:        "",
		cursor:       0,
		historyIndex: 0,
		draft:        "",
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

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

func (m model) View() string {
	var b strings.Builder

	b.WriteString(roseStyle.Render("Roséa Multi-line Editor — Ctrl+S to submit, ESC to quit\n\n"))

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

	return editorStyle.Render(b.String())
}

func main() {
	if err := tea.NewProgram(initialModel()).Start(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
