package ui

import "io"

type UI interface {
	Emitter() Emitter
	Stop()
}

func NewIOUI(w io.Writer) UI {
	writer := newIOWriter(w)
	processor := newProcessor(writer)
	uie := newEmitter(processor)
	return &processedUI{processor, uie}
}

func NewGUI() UI {
	writer := newGUIWriter()
	processor := newProcessor(writer)
	uie := newEmitter(processor)
	return &processedUI{processor, uie}
}

type processedUI struct {
	processor *processor
	uie       Emitter
}

func (pui *processedUI) Emitter() Emitter {
	return pui.uie
}

func (pui *processedUI) Stop() {
	pui.processor.stop()
}
