package core

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/trimble-oss/tierceron-core/v2/trcshfs/trcshio"
)

var roseaProgramCtx tea.Program
var roseaNavigationCtx tea.Model
var roseaEditorCtx tea.Model

var roseaSeedFile string
var roseaMemFs trcshio.MemoryFileSystem

func SetRoseaMemFs(rsf string, rmFs trcshio.MemoryFileSystem) {
	roseaSeedFile = rsf
	roseaMemFs = rmFs
}

func GetRoseaMemFs() (string, trcshio.MemoryFileSystem) { return roseaSeedFile, roseaMemFs }

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

func SanitizePaste(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		return s[1 : len(s)-1]
	}
	return s
}
