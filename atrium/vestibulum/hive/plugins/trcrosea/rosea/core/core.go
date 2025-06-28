package core

import (
	tea "github.com/charmbracelet/bubbletea"
	trcshMemFs "github.com/trimble-oss/tierceron-core/v2/trcshfs"
	"github.com/trimble-oss/tierceron-core/v2/trcshfs/trcshio"
)

var roseaProgramCtx tea.Program
var roseaNavigationCtx tea.Model
var roseaEditorCtx tea.Model

var roseaMemFs trcshio.MemoryFileSystem

func InitMemFs() {
	roseaMemFs = trcshMemFs.NewTrcshMemFs()
}

func GetRoseaMemFs() trcshio.MemoryFileSystem { return roseaMemFs }

func SetRoseaProgramCtx(ctx tea.Program) {
	roseaProgramCtx = ctx
}

func GetRoseaProgramCtx() tea.Program {
	return roseaProgramCtx
}

func SetRoseaNavigationCtx(ctx tea.Model) {
	roseaNavigationCtx = ctx
}

func GetRoseaNavigationCtx() tea.Model {
	return roseaNavigationCtx
}

func SetRoseaEditorCtx(ctx tea.Model) {
	roseaEditorCtx = ctx
}

func GetRoseaEditorCtx() tea.Model {
	return roseaEditorCtx
}
