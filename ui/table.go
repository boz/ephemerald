package ui

import (
	"container/list"
	"strings"

	"github.com/gdamore/tcell"
	"github.com/gdamore/tcell/views"
)

var (
	styleTH = tcell.StyleDefault.Foreground(tcell.ColorWhite)
)

type tuiTH struct {
	value    string
	minwidth int
}

type tuiTD struct {
	value string
	style tcell.Style
}

type tuiTR interface {
	id() string
	cols() []tuiTD
}

type tuiRow struct {
	values []string
	styles []tcell.Style
	views.BoxLayout
}

type tuiTable struct {
	headers []tuiTH
	widths  []int

	wlist *list.List
	wmap  map[string]*list.Element

	content *views.BoxLayout

	views.Panel
}

func newTUITable(headers []tuiTH) *tuiTable {
	t := &tuiTable{
		headers: nil,
		widths:  nil,
		wlist:   list.New(),
		wmap:    make(map[string]*list.Element),
		content: views.NewBoxLayout(views.Vertical),
		Panel:   *views.NewPanel(),
	}

	t.SetContent(t.content)

	t.setHeaders(headers)
	t.insertHeaders()

	return t
}

func (t *tuiTable) setHeaders(headers []tuiTH) {
	t.widths = make([]int, len(headers))
	t.headers = headers

	for idx, header := range t.headers {
		if hl := len(header.value); hl > t.widths[idx] {
			t.widths[idx] = hl
		}
		if header.minwidth > t.widths[idx] {
			t.widths[idx] = header.minwidth
		}
	}
}

func (t *tuiTable) insertHeaders() {
	row := views.NewBoxLayout(views.Horizontal)
	for idx, header := range t.headers {
		value := t.renderCol(header.value, t.widths[idx], false)

		th := views.NewText()
		th.SetText(value)
		th.SetStyle(styleTH)

		row.AddWidget(th, 0.0)
	}

	t.Panel.SetTitle(row)
	t.Resize()
}

func (t *tuiTable) renderCol(value string, width int, preserve bool) string {
	padlen := 1
	vl := len(value)

	if preserve {
		return value
	}

	if vl < width {
		return value + strings.Repeat(" ", width-vl+padlen)
	}
	return value[0:width] + strings.Repeat(" ", padlen)
}

type tuiTRow struct {
	model tuiTR
	views.BoxLayout
}

func (t *tuiTable) handleUpdates(upsert map[string]tuiTR, remove map[string]tuiTR) bool {
	resize := true

	for id, row := range upsert {

		// update
		if e, ok := t.wmap[id]; ok {
			t.handleRowUpdate(row, e)
			continue
		}

		t.handleRowInsert(row)
		resize = true
	}

	for id, _ := range remove {
		t.handleRowDelete(id)
		resize = true
	}

	if resize {
		t.Resize()
	}

	return resize
}

func (t *tuiTable) handleRowUpdate(row tuiTR, e *list.Element) {
	cols := row.cols()

	w := e.Value.(*tuiTRow)
	wcols := w.Widgets()

	for idx, col := range cols {
		wcol := wcols[idx].(*views.Text)
		wcol.SetText(t.renderCol(col.value, t.widths[idx], idx == len(cols)-1))
		wcol.SetStyle(col.style)
		//wcol.Draw()
	}
}

func (t *tuiTable) handleRowInsert(row tuiTR) {
	trow := t.makeTRow(row)

	idx := 0
	e := t.wlist.Front()
	for {

		if e == nil {
			break
		}

		cur := e.Value.(*tuiTRow)
		if strings.Compare(trow.model.id(), cur.model.id()) < 0 {
			t.wmap[row.id()] = t.wlist.InsertBefore(trow, e)
			t.content.InsertWidget(idx, trow, 0.0)
			break
		}

		e = e.Next()
		idx++
	}
	if e == nil {
		t.wmap[row.id()] = t.wlist.PushBack(trow)
		t.content.InsertWidget(idx, trow, 0.0)
	}
}

func (t *tuiTable) makeTRow(row tuiTR) *tuiTRow {
	w := new(tuiTRow)
	w.model = row
	w.BoxLayout = *views.NewBoxLayout(views.Horizontal)
	cols := row.cols()
	for idx, col := range cols {
		wcol := views.NewText()
		wcol.SetText(t.renderCol(col.value, t.widths[idx], idx == len(cols)-1))
		wcol.SetStyle(col.style)
		w.AddWidget(wcol, 0.0)
	}
	w.Resize()
	return w
}

func (t *tuiTable) handleRowDelete(id string) {
	e, ok := t.wmap[id]
	if !ok {
		return
	}
	w := e.Value.(*tuiTRow)
	t.wlist.Remove(e)
	t.content.RemoveWidget(w)
	t.Resize()
	delete(t.wmap, id)
}
