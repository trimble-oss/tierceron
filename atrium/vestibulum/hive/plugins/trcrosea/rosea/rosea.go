package rosea

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	trcshMemFs "github.com/trimble-oss/tierceron-core/v2/trcshfs"
	"github.com/trimble-oss/tierceron-core/v2/trcshfs/trcshio"

	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcrosea/hcore/flowutil"
)

var roseaMemFs trcshio.MemoryFileSystem

var projectServiceMatrix [][]any
var (
	boldStyle    = lipgloss.NewStyle().Bold(true)
	normalStyle  = lipgloss.NewStyle()
	selectedItem = "* "
)

type RoseaModel struct {
	message string
	list    list.Model
	choice  roseaItem
}

type roseaItemDelegate struct{}

func (rid roseaItemDelegate) Height() int                               { return 1 }
func (rid roseaItemDelegate) Spacing() int                              { return 0 }
func (rid roseaItemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (rid roseaItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(roseaItem)
	if !ok {
		return
	}

	str := fmt.Sprintf("%s - %s", i.Title(), i.Description())

	if index == m.Index() {
		fmt.Fprintf(w, boldStyle.Render(selectedItem+str))
	} else {
		fmt.Fprintf(w, "  "+normalStyle.Render(str))
	}
}

const defaultListWidth = 20
const defaultListHeight = 10

type roseaItem struct {
	title, desc string
}

func (i roseaItem) Title() string       { return i.title }
func (i roseaItem) Description() string { return i.desc }
func (i roseaItem) FilterValue() string { return i.title }

func (rm *RoseaModel) Init() tea.Cmd {
	fmt.Print("\033[H\033[2J")
	fmt.Println("Rosea Editor")
	if roseaMemFs == nil {
		roseaMemFs = trcshMemFs.NewTrcshMemFs()
	}

	roseaItems := []list.Item{}
	for _, pluginProjectService := range projectServiceMatrix {
		roseaItems = append(roseaItems, roseaItem{title: pluginProjectService[1].(string)})
	}

	rm.list = list.New(roseaItems, roseaItemDelegate{}, defaultListWidth, defaultListHeight)

	return nil // No initial commands needed
}

func (rm *RoseaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyDown:
			rm.list.CursorDown()
			if selectedItem, ok := rm.list.SelectedItem().(roseaItem); ok {
				rm.choice = selectedItem
			}
		case tea.KeyUp:
			rm.list.CursorUp()
			if selectedItem, ok := rm.list.SelectedItem().(roseaItem); ok {
				rm.choice = selectedItem
			}
		default:
			switch msg.String() {
			case "ctrl+c", "q":
				return rm, tea.Quit
			case "enter":
				if selectedItem, ok := rm.list.SelectedItem().(roseaItem); ok {
					// TODO: load editor for selected item.
					chatResponseMsg := tccore.CallChatQueryChan(flowutil.GetChatMsgHookCtx(),
						"rosea", // From rainier
						&tccore.TrcdbExchange{
							Flows:     []string{flowcore.ArgosSociiFlow.TableName()},                                                                                                          // Flows
							Query:     fmt.Sprintf("SELECT * FROM %s.%s WHERE argosIdentitasNomen='%s'", flowutil.GetDatabaseName(), flowcore.ArgosSociiFlow.TableName(), selectedItem.title), // Query
							Operation: "SELECT",                                                                                                                                               // query operation
							ExecTrcsh: "/edit/load.trc.tmpl",
							Request: tccore.TrcdbRequest{
								Rows: [][]any{
									{roseaMemFs},
								},
							},
						},
						flowutil.GetChatSenderChan(),
					)
					if chatResponseMsg.TrcdbExchange != nil && len(chatResponseMsg.TrcdbExchange.Response.Rows) > 0 {
						entrySeedFs := chatResponseMsg.TrcdbExchange.Request.Rows[0][0].(trcshio.MemoryFileSystem)
						if entrySeedFs != nil {
							// TODO: get this into an editor.
						}
					}

					rm.choice = selectedItem
					return rm, tea.ClearScreen
				}
			}
		}
	}
	return rm, nil
}

func (rm *RoseaModel) View() string {
	bold := lipgloss.NewStyle().Bold(true)

	s := "Plugins managed by tierceron:\n\n"
	for _, item := range rm.list.Items() {
		rItem := item.(roseaItem)
		if rItem.title == rm.choice.title {
			s += fmt.Sprintf("%s\n", bold.Render(fmt.Sprintf("> %s", rItem.title)))
		} else {
			s += fmt.Sprintf("%s\n", rItem.title)
		}
	}

	s += "\nPress ↑/↓ to navigate, 'q' or Ctrl+C to exit"

	return s
}

var roseaProgramCtx *tea.Program

func BootInit(psm [][]any) error {
	projectServiceMatrix = psm
	roseaProgramCtx = tea.NewProgram(&RoseaModel{})
	_, err := roseaProgramCtx.Run()
	return err
}
