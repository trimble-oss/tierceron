//go:build trcshkernelz && fyneboot

package shell

import (
	tea "github.com/charmbracelet/bubbletea"
	fenestracore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcfenestra/hcore"
)

func (m *ShellModel) launchFenestraPOC() tea.Cmd {
	return func() tea.Msg {
		if err := fenestracore.LaunchPOCWindow(); err != nil {
			return commandResultMsg{
				output:     []string{errorStyle.Render(err.Error()), ""},
				shouldQuit: false,
			}
		}

		return commandResultMsg{
			output:     []string{"Launching trcfenestra POC window...", ""},
			shouldQuit: false,
		}
	}
}
