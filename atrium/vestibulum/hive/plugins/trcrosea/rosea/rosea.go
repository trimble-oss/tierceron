package rosea

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	trcshMemFs "github.com/trimble-oss/tierceron-core/v2/trcshfs"
	"github.com/trimble-oss/tierceron-core/v2/trcshfs/trcshio"

	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcrosea/hcore/flowutil"
	roseacore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcrosea/rosea/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcrosea/testr"
)

var roseaMemFs trcshio.MemoryFileSystem

var projectServiceMatrix [][]any
var (
	boldStyle    = lipgloss.NewStyle().Bold(true)
	normalStyle  = lipgloss.NewStyle()
	selectedItem = "* "
)

type RoseaModel struct {
	message    string
	list       list.Model
	roseaItems []list.Item
	choice     roseaItem
	pageSize   int
	pageIndex  int
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

func (m *RoseaModel) updateListItems() {
	start := m.pageIndex * m.pageSize
	end := start + m.pageSize
	if end > len(m.roseaItems) {
		end = len(m.roseaItems)
	}
	m.list.SetItems(m.roseaItems[start:end])
}

func (rm *RoseaModel) Init() tea.Cmd {
	fmt.Print("\033[H\033[2J")
	if roseaMemFs == nil {
		roseaMemFs = trcshMemFs.NewTrcshMemFs()
	}

	for _, pluginProjectService := range projectServiceMatrix {
		rm.roseaItems = append(rm.roseaItems, roseaItem{title: pluginProjectService[1].(string)})
	}

	rm.list = list.New([]list.Item{}, roseaItemDelegate{}, defaultListWidth, defaultListHeight)

	rm.updateListItems()
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
		case tea.KeyRight:
			if (rm.pageIndex+1)*rm.pageSize < len(rm.roseaItems) {
				rm.pageIndex++
				rm.updateListItems()
			}
		case tea.KeyLeft:
			if rm.pageIndex > 0 {
				rm.pageIndex--
				rm.updateListItems()
			}
		default:
			switch msg.String() {
			case "ctrl+c", "q":
				return rm, tea.Quit
			case "enter":
				if selectedItem, ok := rm.list.SelectedItem().(roseaItem); ok {
					roseaMemFs = trcshMemFs.NewTrcshMemFs()
					chatResponseMsg := tccore.CallChatQueryChan(flowutil.GetChatMsgHookCtx(),
						"rosea", // From rainier
						&tccore.TrcdbExchange{
							Flows:     []string{flowcore.ArgosSociiFlow.TableName()},                                                                                    // Flows
							Query:     fmt.Sprintf("SELECT * FROM %s.%s WHERE argosIdentitasNomen='%s'", "%s", flowcore.ArgosSociiFlow.TableName(), selectedItem.title), // Query
							Operation: "SELECT",                                                                                                                         // query operation
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
							seedFilePath := ""
							// TODO: get this into an editor.
							entrySeedFs.Walk("./trc_seeds", func(p string, isDir bool) error {
								if !isDir && strings.HasSuffix(p, ".yml") && len(seedFilePath) == 0 {
									seedFilePath = p
								}
								return nil
							})
							if len(seedFilePath) > 0 {
								entrySeedFileRWC, err := entrySeedFs.Open(seedFilePath)
								if err != nil {
									fmt.Printf("Error opening seed file: %v\n", err)
									return rm, nil
								}
								fileData, _ := io.ReadAll(entrySeedFileRWC)
								return testr.InitRoseaEditor(&fileData), nil
							}
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

	s += "\nPress ↑/↓ to navigate, ←/→ to paginate, 'q' or Ctrl+C to exit"

	return s
}

var roseaProgramCtx *tea.Program
var roseaNavigationCtx *RoseaModel

func GetRoseaNavigationCtx() *RoseaModel {
	if roseaNavigationCtx == nil {
		roseaNavigationCtx = &RoseaModel{}
	}
	return roseaNavigationCtx
}

func BootInit(psm [][]any) error {
	projectServiceMatrix = psm
	roseacore.SetRoseaNavigationCtx(&RoseaModel{pageSize: 10, pageIndex: 0})
	roseaProgramCtx = tea.NewProgram(roseacore.GetRoseaNavigationCtx())
	_, err := roseaProgramCtx.Run()
	return err
}
