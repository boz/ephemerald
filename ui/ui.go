package ui

import "io"

type UI interface {
	Emitter() Emitter
	Stop()
}

func NewIOUI(w io.Writer) (UI, error) {
	writer := newIOWriter(w)
	processor := newProcessor(writer)
	uie := newEmitter(processor)
	return &processedUI{processor, uie}, nil
}

func NewTUI(donech chan bool) (UI, error) {
	writer, err := newTUIWriter(donech)
	if err != nil {
		return nil, err
	}

	processor := newProcessor(writer)
	uie := newEmitter(processor)
	return &processedUI{processor, uie}, nil
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
