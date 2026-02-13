package dirpicker

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DirSelectedMsg is sent when a directory is selected
type DirSelectedMsg struct {
	Path string
}

// DirPickerCancelMsg is sent when the picker is cancelled
type DirPickerCancelMsg struct{}

type DirPickerModel struct {
	currentPath string
	entries     []os.DirEntry
	cursor      int
	width       int
	height      int
	selected    bool
	cancelled   bool
}

var (
	titleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#eb6f92")).Bold(true)
	dirStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ccfd8"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#9ccfd8"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#f6c177"))
	pathStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebbcba"))
)

func NewDirPicker(startPath string) *DirPickerModel {
	if startPath == "" {
		var err error
		startPath, err = os.Getwd()
		if err != nil {
			startPath = "/"
		}
	}

	m := &DirPickerModel{
		currentPath: startPath,
		width:       80,
		height:      24,
	}

	m.loadEntries()
	return m
}

func (m *DirPickerModel) loadEntries() {
	entries, err := os.ReadDir(m.currentPath)
	if err != nil {
		// If we can't read, go to parent
		parent := filepath.Dir(m.currentPath)
		if parent != m.currentPath {
			m.currentPath = parent
			m.loadEntries()
		}
		return
	}

	// Filter to only show directories
	dirs := []os.DirEntry{}
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry)
		}
	}

	// Sort directories alphabetically
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name()) < strings.ToLower(dirs[j].Name())
	})

	// Add parent directory entry if not at root
	if m.currentPath != "/" {
		m.entries = append([]os.DirEntry{&parentDirEntry{}}, dirs...)
	} else {
		m.entries = dirs
	}

	// Reset cursor if it's out of bounds
	if m.cursor >= len(m.entries) {
		m.cursor = 0
	}
}

// parentDirEntry is a fake entry for ".."
type parentDirEntry struct{}

func (p *parentDirEntry) Name() string               { return ".." }
func (p *parentDirEntry) IsDir() bool                { return true }
func (p *parentDirEntry) Type() os.FileMode          { return os.ModeDir }
func (p *parentDirEntry) Info() (os.FileInfo, error) { return nil, nil }

func (m *DirPickerModel) Init() tea.Cmd {
	return nil
}

func (m *DirPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit

		case "enter":
			if len(m.entries) == 0 {
				return m, nil
			}

			entry := m.entries[m.cursor]
			if entry.Name() == ".." {
				// Go to parent directory
				m.currentPath = filepath.Dir(m.currentPath)
				m.loadEntries()
				m.cursor = 0
			} else {
				// Enter subdirectory or select
				newPath := filepath.Join(m.currentPath, entry.Name())
				if msg.Alt {
					// Alt+Enter to select current directory without entering
					m.selected = true
					return m, tea.Quit
				} else {
					// Enter to navigate into directory
					m.currentPath = newPath
					m.loadEntries()
					m.cursor = 0
				}
			}

		case "s":
			// 's' key to select current directory
			m.selected = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}

		case "home", "g":
			m.cursor = 0

		case "end", "G":
			m.cursor = len(m.entries) - 1

		case "pgup":
			m.cursor -= 10
			if m.cursor < 0 {
				m.cursor = 0
			}

		case "pgdown":
			m.cursor += 10
			if m.cursor >= len(m.entries) {
				m.cursor = len(m.entries) - 1
			}
		}
	}

	return m, nil
}

func (m *DirPickerModel) View() string {
	if m.selected || m.cancelled {
		return ""
	}

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("Select Output Directory"))
	b.WriteString("\n\n")

	// Current path
	b.WriteString(pathStyle.Render("Current: " + m.currentPath))
	b.WriteString("\n\n")

	// Directory listing
	visibleHeight := m.height - 8
	start := 0
	end := len(m.entries)

	// Scroll to keep cursor visible
	if len(m.entries) > visibleHeight {
		if m.cursor > visibleHeight/2 {
			start = m.cursor - visibleHeight/2
			end = start + visibleHeight
			if end > len(m.entries) {
				end = len(m.entries)
				start = end - visibleHeight
				if start < 0 {
					start = 0
				}
			}
		} else {
			end = visibleHeight
		}
	}

	for i := start; i < end && i < len(m.entries); i++ {
		entry := m.entries[i]
		prefix := "  "
		entryName := entry.Name()

		if i == m.cursor {
			prefix = "► "
			if entry.Name() == ".." {
				b.WriteString(selectedStyle.Render(prefix + entryName))
			} else {
				b.WriteString(selectedStyle.Render(prefix + "📁 " + entryName))
			}
		} else {
			if entry.Name() == ".." {
				b.WriteString(dirStyle.Render(prefix + entryName))
			} else {
				b.WriteString(dirStyle.Render(prefix + "📁 " + entryName))
			}
		}
		b.WriteString("\n")
	}

	// Help text
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Navigate: ↑/↓ or j/k  |  Enter: open directory  |  s: select current directory"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Esc: cancel"))

	return b.String()
}

// PickDirectory runs the directory picker and returns the selected path
func PickDirectory(startPath string) (string, error) {
	m := NewDirPicker(startPath)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	final := finalModel.(*DirPickerModel)
	if final.cancelled {
		return "", fmt.Errorf("directory selection cancelled")
	}

	return final.currentPath, nil
}
