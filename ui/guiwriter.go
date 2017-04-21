package ui

import (
	"container/list"
	"fmt"
	"strconv"
	"strings"
	"time"

	throttle "github.com/boz/go-throttle"
	"github.com/buger/goterm"
)

const (
	guiMaxPeriod = time.Second / 2
	guiMinPeriod = time.Second / 15
)

type crec struct {
	c   container
	key string
}

type guiWriter struct {
	clist *list.List
	cmap  map[string]*list.Element

	plist *list.List
	pmap  map[string]*list.Element

	pch chan pool
	cch chan container

	// channel for deleting containers
	dch chan container

	drawch chan bool

	throttle throttle.ThrottleDriver

	donech chan bool
}

func newGUIWriter() writer {
	writer := &guiWriter{
		clist: list.New(),
		cmap:  make(map[string]*list.Element),

		plist: list.New(),
		pmap:  make(map[string]*list.Element),

		pch: make(chan pool),
		cch: make(chan container),
		dch: make(chan container),

		drawch: make(chan bool),

		donech: make(chan bool),
	}

	writer.throttle = throttle.ThrottleFunc(guiMinPeriod, true, func() {
		select {
		case <-writer.donech:
		case writer.drawch <- true:
		}
	})

	go writer.run()
	return writer
}

func (w *guiWriter) createPool(p pool) {
	select {
	case <-w.donech:
	case w.pch <- p:
	}
}

func (w *guiWriter) updatePool(p pool) {
	select {
	case <-w.donech:
	case w.pch <- p:
	}
}

func (w *guiWriter) createContainer(c container) {
	select {
	case <-w.donech:
	case w.cch <- c:
	}
}

func (w *guiWriter) updateContainer(c container) {
	select {
	case <-w.donech:
	case w.cch <- c:
	}
}

func (w *guiWriter) deleteContainer(c container) {
	select {
	case <-w.donech:
	case w.dch <- c:
	}
}

func (w *guiWriter) stop() {
	w.throttle.Stop()
	close(w.donech)
}

func (w *guiWriter) run() {
	for {
		select {
		case <-w.donech:
			return
		case <-w.drawch:
			w.draw()
		case p := <-w.pch:
			w.handlePool(p)
			w.throttle.Trigger()
		case c := <-w.cch:
			w.handleContainer(c)
			w.throttle.Trigger()
		case c := <-w.dch:
			w.handleDeleteContainer(c)
			w.throttle.Trigger()
		case <-time.After(guiMaxPeriod):
		}
	}
}

func (w *guiWriter) handlePool(p pool) {
	pel := w.pmap[p.name]
	if pel != nil {
		pel.Value = p
		return
	}

	for e := w.plist.Front(); e != nil; e = e.Next() {
		cur := e.Value.(pool)
		if strings.Compare(p.name, cur.name) < 0 {
			w.pmap[p.name] = w.plist.InsertBefore(p, e)
			return
		}
	}
	w.pmap[p.name] = w.plist.PushBack(p)
}

func (w *guiWriter) handleContainer(c container) {
	cel := w.cmap[c.id]
	if cel != nil {
		cel.Value.(*crec).c = c
		return
	}

	key := c.pname + c.id

	for e := w.clist.Front(); e != nil; e = e.Next() {
		cur := e.Value.(*crec)
		if strings.Compare(key, cur.key) < 0 {
			w.cmap[c.id] = w.clist.InsertBefore(&crec{c, key}, e)
			return
		}
	}
	w.cmap[c.id] = w.clist.PushBack(&crec{c, key})
}

func (w *guiWriter) handleDeleteContainer(c container) {
	cel := w.cmap[c.id]
	if cel != nil {
		w.clist.Remove(cel)
		delete(w.cmap, c.id)
	}
}

const (
	guiPHeader      = "---=[ Pools ]=---"
	guiCHeader      = "---=[ Containers ]=---"
	maxLifecycleLen = len("healthcheck")
	maxActionLen    = len("postgres.truncate")
)

func (w *guiWriter) clearScreen() {
	goterm.Clear()
	height := goterm.Height()
	goterm.MoveCursor(height, 1)
	goterm.Flush()
}

func (w *guiWriter) draw() {
	goterm.Clear()

	height := goterm.Height()

	contentRows := len(w.pmap) + len(w.cmap)
	borderRows := 9
	totalRows := borderRows + contentRows

	goterm.MoveCursor(height-totalRows, 1)

	w.drawContainers()
	w.drawPools()

	goterm.Flush()
}

func (w *guiWriter) drawPools() {
	goterm.Println(goterm.Bold(guiPHeader))
	goterm.Println("")

	tab := &table{
		header: []cell{
			{"Name", goterm.WHITE, 0},
			{"State", goterm.WHITE, pstateMaxLen},
			{"Ready", goterm.WHITE, 3},
			{"Total", goterm.WHITE, 3},
			{"Pending", goterm.WHITE, 3},
			{"Error", goterm.WHITE, 0},
		},
	}

	for e := w.plist.Front(); e != nil; e = e.Next() {
		cur := e.Value.(pool)

		errval := ""
		errcolor := 0
		if cur.err != nil {
			errval = fmt.Sprint(cur.err)
			errcolor = goterm.RED
		}

		statecolor := 0
		switch cur.state {
		case pstateRunning:
			statecolor = goterm.GREEN
		case pstateDraining:
			statecolor = goterm.YELLOW
		case pstateErr:
			statecolor = goterm.RED
		}

		tab.rows = append(tab.rows, []cell{
			{cur.name, 0, 0},
			{string(cur.state), statecolor, 0},
			{strconv.Itoa(cur.numReady), 0, 0},
			{strconv.Itoa(cur.numItems), 0, 0},
			{strconv.Itoa(cur.numPending), 0, 0},
			{errval, errcolor, 0},
		})
	}

	goterm.Println(tab)
}

func (w *guiWriter) drawContainers() {
	goterm.Println(goterm.Bold(guiCHeader))
	goterm.Println("")

	tab := &table{
		header: []cell{
			{"Pool", goterm.WHITE, 5},
			{"ID", goterm.WHITE, 12},
			{"State", goterm.WHITE, cstateMaxLen},
			{"Lifecycle", goterm.WHITE, maxLifecycleLen},
			{"Action", goterm.WHITE, maxActionLen},
			{"Attempt", goterm.WHITE, 5},
			{"Error", goterm.WHITE, 0},
		},
	}

	for e := w.clist.Front(); e != nil; e = e.Next() {
		cur := e.Value.(*crec).c

		scolor := goterm.YELLOW
		switch cur.state {
		case cstateReady:
			scolor = goterm.GREEN
		case cstateExiting, cstateExited:
			scolor = goterm.RED
		}

		row := []cell{
			{cur.pname, 0, 0},
			{cur.id[0:12], 0, 0},
			{string(cur.state), scolor, 0},
			{cur.lifecycleName, 0, 0},
			{cur.actionName, 0, 0},
		}

		if cur.actionName == "" {
			row = append(row, cell{"", 0, 0}) // attempt
			row = append(row, cell{"", 0, 0}) // error
		} else {

			row = append(row, cell{fmt.Sprintf("[%v/%v]", cur.actionAttempt, cur.actionAttempts), 0, 0})

			if cur.actionError == nil {
				row = append(row, cell{"", 0, 0})
			} else {
				row = append(row, cell{fmt.Sprint(cur.actionError), goterm.WHITE, 0})
			}
		}
		tab.rows = append(tab.rows, row)
	}
	goterm.Println(tab)
}
