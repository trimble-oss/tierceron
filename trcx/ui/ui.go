package ui

import (
	xdb "tierceron/trcx/db"

	"fyne.io/fyne"
	"fyne.io/fyne/widget"
)

func StartAndRunUI(w fyne.Window, tierceronEngine *xdb.TierceronEngine) {
	w.SetContent(
		makeTable(
			[]string{"Foo is much longer", "Bar", "Baz"},
			[][]string{{"1", "2", "3"}, {"4", "5", "6"}},
		),
	)
}

func makeTable(headings []string, rows [][]string) *widget.Box {

	columns := rowsToColumns(headings, rows)

	objects := make([]fyne.CanvasObject, len(columns))
	for k, col := range columns {
		box := widget.NewVBox(widget.NewLabelWithStyle(headings[k], fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		for _, val := range col {
			box.Append(widget.NewLabel(val))
		}
		objects[k] = box
	}
	return widget.NewHBox(objects...)
}

func rowsToColumns(headings []string, rows [][]string) [][]string {
	columns := make([][]string, len(headings))
	for _, row := range rows {
		for colK := range row {
			columns[colK] = append(columns[colK], row[colK])
		}
	}
	return columns
}
