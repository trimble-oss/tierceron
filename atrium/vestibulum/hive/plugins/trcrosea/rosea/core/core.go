package core

import (
	tea "github.com/charmbracelet/bubbletea"
)

var roseaProgramCtx tea.Program
var roseaNavigationCtx tea.Model
var roseaEditorCtx tea.Model

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
