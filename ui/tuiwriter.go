package ui

import (
	"fmt"
	"strconv"
	"time"

	throttle "github.com/boz/go-throttle"
	"github.com/gdamore/tcell"
	"github.com/gdamore/tcell/views"
)

const (
	tuiMaxPeriod = time.Second / 2
	tuiMinPeriod = time.Second / 15
)

type crec struct {
	c   container
	key string
}

type tuiWriter struct {
	cupdate map[string]tuiTR
	cdelete map[string]tuiTR
	pupdate map[string]tuiTR
	pdelete map[string]tuiTR

	pch chan pool
	cch chan container

	// channel for deleting containers
	dch chan container

	drawch chan bool

	throttle throttle.ThrottleDriver

	// closed when user quits.
	shutdownch chan bool

	donech chan bool

	app    *views.Application
	window *tuiWindow
}

func newTUIWriter(shutdownch chan bool) (writer, error) {

	app, window := createTuiApp(shutdownch)

	writer := &tuiWriter{
		cupdate: make(map[string]tuiTR),
		cdelete: make(map[string]tuiTR),
		pupdate: make(map[string]tuiTR),
		pdelete: make(map[string]tuiTR),

		pch: make(chan pool),
		cch: make(chan container),
		dch: make(chan container),

		drawch: make(chan bool),

		shutdownch: shutdownch,
		donech:     make(chan bool),

		app:    app,
		window: window,
	}

	writer.throttle = throttle.ThrottleFunc(tuiMinPeriod, true, func() {
		select {
		case <-writer.donech:
		case writer.drawch <- true:
		}
	})

	go app.Run()

	go writer.run()
	return writer, nil
}

func (w *tuiWriter) createPool(p pool) {
	select {
	case <-w.donech:
	case w.pch <- p:
	}
}

func (w *tuiWriter) updatePool(p pool) {
	select {
	case <-w.donech:
	case w.pch <- p:
	}
}

func (w *tuiWriter) createContainer(c container) {
	select {
	case <-w.donech:
	case w.cch <- c:
	}
}

func (w *tuiWriter) updateContainer(c container) {
	select {
	case <-w.donech:
	case w.cch <- c:
	}
}

func (w *tuiWriter) deleteContainer(c container) {
	select {
	case <-w.donech:
	case w.dch <- c:
	}
}

func (w *tuiWriter) stop() {
	w.throttle.Stop()
	close(w.donech)
}

func (w *tuiWriter) run() {
	defer w.app.Quit()
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
		case <-time.After(tuiMaxPeriod):
		}
	}
}

func (w *tuiWriter) handlePool(p pool) {
	tr := newPoolTRow(p)
	w.pupdate[tr.id()] = tr
}

func (w *tuiWriter) handleContainer(c container) {
	cr := newContainerTRow(c)
	w.cupdate[cr.id()] = cr
}

func (w *tuiWriter) handleDeleteContainer(c container) {
	cr := newContainerTRow(c)
	w.cdelete[cr.id()] = cr
}

func (w *tuiWriter) draw() {

	cupdate := w.cupdate
	cdelete := w.cdelete
	pupdate := w.pupdate
	pdelete := w.pdelete

	w.cupdate = make(map[string]tuiTR)
	w.cdelete = make(map[string]tuiTR)

	w.pupdate = make(map[string]tuiTR)
	w.pdelete = make(map[string]tuiTR)

	w.app.PostFunc(func() {
		w.window.updateContainers(cupdate, cdelete)
		w.window.updatePools(pupdate, pdelete)
		w.window.Resize()
		w.window.Draw()
	})

}

type poolTRow struct {
	value pool
}

func newPoolTRow(value pool) poolTRow {
	return poolTRow{value}
}

func (pr poolTRow) id() string {
	return pr.value.name
}

func (pr poolTRow) cols() []tuiTD {
	style := tcell.StyleDefault
	statecolor := tcell.StyleDefault
	errcolor := tcell.StyleDefault
	errval := ""

	switch pr.value.state {
	case pstateRunning:
		statecolor = statecolor.Foreground(tcell.ColorGreen)
	case pstateDraining:
		statecolor = statecolor.Foreground(tcell.ColorYellow)
	case pstateErr:
		statecolor = statecolor.Foreground(tcell.ColorRed)
	}

	if pr.value.err != nil {
		errval = fmt.Sprint(pr.value.err)
		errcolor = errcolor.Foreground(tcell.ColorRed)
	}

	cols := []tuiTD{
		{pr.value.name, style},
		{string(pr.value.state), statecolor},
		{strconv.Itoa(pr.value.numReady), style},
		{strconv.Itoa(pr.value.numItems), style},
		{strconv.Itoa(pr.value.numPending), style},
		{errval, errcolor},
	}
	return cols
}

type containerTRow struct {
	key   string
	value container
}

func newContainerTRow(value container) containerTRow {
	return containerTRow{value.pname + value.id, value}
}

func (cr containerTRow) id() string {
	return cr.key
}

func (cr containerTRow) cols() []tuiTD {
	style := tcell.StyleDefault

	cols := []tuiTD{
		{cr.value.pname, style},
		{cr.value.id[0:12], style},
	}

	style = tcell.StyleDefault

	switch cr.value.state {
	case cstateReady:
		style = style.Foreground(tcell.ColorGreen)
	case cstateExiting, cstateExited:
		style = style.Foreground(tcell.ColorRed)
	default:
		style = style.Foreground(tcell.ColorYellow)
	}

	cols = append(cols, tuiTD{string(cr.value.state), style})

	style = tcell.StyleDefault

	cols = append(cols, tuiTD{cr.value.lifecycleName, style})
	cols = append(cols, tuiTD{cr.value.actionName, style})

	if cr.value.actionName == "" {
		cols = append(cols, tuiTD{"", style}) // attempt
		cols = append(cols, tuiTD{"", style}) // error
	} else {
		val := fmt.Sprintf("[%v/%v]", cr.value.actionAttempt, cr.value.actionAttempts)
		cols = append(cols, tuiTD{val, style})

		if cr.value.actionError == nil {
			cols = append(cols, tuiTD{"", style})
		} else {
			cols = append(cols, tuiTD{fmt.Sprint(cr.value.actionError), style.Foreground(tcell.ColorWhite)})
		}
	}

	return cols
}
