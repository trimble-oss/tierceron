//go:build !trcshkernelz || !fyneboot

package shell

import tea "github.com/charmbracelet/bubbletea"

func (m *ShellModel) launchFenestraPOC() tea.Cmd {
	return func() tea.Msg {
		return commandResultMsg{
			output: []string{
				errorStyle.Render("Error: trcfenestra is only available in trcshz builds with fyneboot enabled"),
				"",
			},
			shouldQuit: false,
		}
	}
}
