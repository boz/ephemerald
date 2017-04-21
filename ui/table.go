package ui

import (
	"bytes"
	"strings"

	"github.com/buger/goterm"
)

// FFS.

type table struct {
	header []cell
	rows   [][]cell
}

type cell struct {
	value    string
	color    int
	minwidth int
}

func (c *cell) String() string {
	if c.color == 0 {
		return string(c.value)
	} else {
		return goterm.Color(string(c.value), c.color)
	}
}

func (t *table) String() string {

	widths := make([]int, len(t.header))

	for i, cell := range t.header {
		widths[i] = len(cell.value)
		if widths[i] < cell.minwidth {
			widths[i] = cell.minwidth
		}
	}

	for _, row := range t.rows {
		for i, cell := range row {
			if widths[i] < len(cell.value) {
				widths[i] = len(cell.value)
			}
		}
	}

	buf := new(bytes.Buffer)

	padding := 3

	for i, cell := range t.header {
		buf.WriteString(cell.String())
		if i != len(t.header)-1 {
			buf.WriteString(strings.Repeat(" ", (widths[i]-len(cell.value))+padding))
		}
	}
	buf.WriteString("\n")

	for _, row := range t.rows {
		for i, cell := range row {
			buf.WriteString(cell.String())
			if i != len(row)-1 {
				buf.WriteString(strings.Repeat(" ", (widths[i]-len(cell.value))+padding))
			}
		}
		buf.WriteString("\n")
	}

	return buf.String()
}
