package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

type MyTable struct {
	*widget.Table
	CurrentRow int

	OnActivated func(fyne.Shortcut)
}

func NewMyTable(length func() (rows int, cols int), create func() fyne.CanvasObject, update func(widget.TableCellID, fyne.CanvasObject)) *MyTable {
	table := &MyTable{
		Table: widget.NewTable(length, create, update),
	}

	table.moveSelection(0)
	return table
}

func (t *MyTable) moveSelection(diff int) {
	if t.CurrentRow+diff < 0 {
		return
	}
	if f := t.Length; f != nil {
		rows, _ := f()
		if t.CurrentRow+diff > rows-1 {
			return
		}
	}

	t.RefreshItem(widget.TableCellID{Row: t.CurrentRow, Col: 1})
	t.CurrentRow += diff
	t.ScrollTo(widget.TableCellID{Row: t.CurrentRow, Col: 1})
	t.RefreshItem(widget.TableCellID{Row: t.CurrentRow, Col: 1})
	t.Select(widget.TableCellID{Row: t.CurrentRow, Col: 1})
}

func (t *MyTable) Refresh() {
	t.Table.Refresh()
	t.Select(widget.TableCellID{Row: t.CurrentRow, Col: 1})
}

func (t *MyTable) TypedKey(event *fyne.KeyEvent) {
	switch event.Name {
	case fyne.KeyReturn, fyne.KeyEnter:
		if t.OnActivated != nil {
			t.OnActivated(nil)
		}
	case fyne.KeyUp:
		t.moveSelection(-1)
	case fyne.KeyDown:
		t.moveSelection(1)
	}
}

func (t *MyTable) AddHeader(cols ...string) {
	t.ShowHeaderRow = true
	t.CreateHeader = func() fyne.CanvasObject {
		label := widget.NewLabel("Column")
		label.TextStyle.Bold = true
		label.Alignment = fyne.TextAlignCenter
		return label
	}
	t.UpdateHeader = func(id widget.TableCellID, template fyne.CanvasObject) {
		label := template.(*widget.Label)
		if id.Col < len(cols) {
			label.SetText(cols[id.Col])
		} else {
			label.SetText("Col missing")
		}
	}
}

func (t *MyTable) SetColWidths(widths ...float32) {
	for c, w := range widths {
		t.SetColumnWidth(c, w)
	}
}
