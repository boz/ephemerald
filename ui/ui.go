package ui

import "io"

type UI interface {
	Emitter() Emitter
	Stop()
}

func NewIOUI(w io.Writer) UI {
	writer := newIOWriter(w)
	processor := newProcessor(writer)
	emitter := newEmitter(processor)
	return &processedUI{processor, emitter}
}

func NewGUI() UI {
	writer := newGUIWriter()
	processor := newProcessor(writer)
	emitter := newEmitter(processor)
	return &processedUI{processor, emitter}
}

type processedUI struct {
	processor *processor
	emitter   Emitter
}

func (pui *processedUI) Emitter() Emitter {
	return pui.emitter
}

func (pui *processedUI) Stop() {
	pui.processor.stop()
}
