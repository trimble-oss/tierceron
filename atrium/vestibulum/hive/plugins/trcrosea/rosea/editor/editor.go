package testr

import (
	"bytes"
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

// getClipboardContent retrieves clipboard content on Linux and WSL
func getClipboardContent() string {
	// Try WSL first (PowerShell Get-Clipboard)
	if content, err := tryWSLClipboard(); err == nil && content != "" {
		return content
	}

	// Try Wayland (wl-paste)
	if content, err := tryWaylandClipboard(); err == nil && content != "" {
		return content
	}

	// Try X11 with xclip
	if content, err := tryX11ClipboardXclip(); err == nil && content != "" {
		return content
	}

	// Try X11 with xsel
	if content, err := tryX11ClipboardXsel(); err == nil && content != "" {
		return content
	}

	return ""
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

func InitRoseaEditor(title string, data *[]byte) *RoseaEditorModel {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width = 80
		height = 24
	}

	initialContent := strings.Join(lines(data), "\n")
	return &RoseaEditorModel{
		title:         title,
		width:         width,
		height:        height,
		lines:         []string{},
		input:         initialContent, // Initialize input with existing lines
		cursor:        0,
		cursorVisible: true,
		historyIndex:  0,
		draft:         "",
		editorStyle:   baseStyle.Padding(1, 2).Width(width),
		roseaMode:     true, // Enable rosea mode for direct memfs save
		modified:      false,
	}
}

func (m *RoseaEditorModel) Init() tea.Cmd {
	return cursorBlink()
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
			return m, nil

		case tea.KeyCtrlS: // Submit on Ctrl+S (also saves)
			if m.roseaMode {
				// Rosea mode: save directly to memfs without auth popup
				filename, memfs := roseacore.GetRoseaMemFs()
				if memfs != nil {
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
			return m, nil

		case tea.KeyBackspace:
			if m.cursor > 0 && len(m.input) > 0 {
				m.input = m.input[:m.cursor-1] + m.input[m.cursor:]
				m.cursor--
				m.modified = true
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
				}
			} else if msg.Type == tea.KeySpace {
				if m.showAuthPopup {
					m.authInput = m.authInput[:m.authCursor] + " " + m.authInput[m.authCursor:]
					m.authCursor++
				} else {
					m.input = m.input[:m.cursor] + " " + m.input[m.cursor:]
					m.cursor++
					m.modified = true
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

func (m *RoseaEditorModel) View() string {
	var b strings.Builder

	b.WriteString(roseStyle.Render("Roséa Multi-line Editor"))
	b.WriteString("\n")

	for _, line := range m.lines {
		b.WriteString(pineStyle.Render(line) + "\n")
	}

	// Render input with cursor
	lines := strings.Split(m.input, "\n")
	visibleHeight := m.height - 4
	start := m.scrollOffset
	end := min(len(lines), start+visibleHeight)

	row, col := cursorRowCol(m.input, m.cursor)
	for i := start; i < end; i++ {
		line := lines[i]
		if i > start {
			b.WriteString("\n")
		}
		if i == row {
			// Current line with cursor - render with uniform styling
			b.WriteString(foamStyle.Render(line[:col]))
			if m.cursorVisible {
				if col < len(line) {
					cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#c4a7e7"))
					b.WriteString(cursorStyle.Render(string(line[col])))
					b.WriteString(foamStyle.Render(line[col+1:]))
				} else {
					cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#c4a7e7"))
					b.WriteString(cursorStyle.Render(" "))
				}
			} else {
				b.WriteString(foamStyle.Render(line[col:]))
			}
		} else {
			b.WriteString(foamStyle.Render(line))
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

	return m.editorStyle.Width(m.width).Render(b.String())
}

// func main() {
// 	if err := tea.NewProgram(initialModel(nil)).Start(); err != nil {
// 		fmt.Fprintln(os.Stderr, "Error:", err)
// 		os.Exit(1)
// 	}
// }
