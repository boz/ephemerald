package ephemerald

import "github.com/boz/ephemerald/ui"

type poolItemBuffer interface {
	get() <-chan poolItem
	put(c poolItem)
	stop()
}

type pibuffer struct {
	outch chan poolItem
	inch  chan poolItem
	buf   []poolItem

	uie ui.PoolEmitter
}

func newPoolItemBuffer(uie ui.PoolEmitter) poolItemBuffer {
	b := &pibuffer{
		outch: make(chan poolItem),
		inch:  make(chan poolItem),
		uie:   uie,
	}
	go b.run()
	return b
}

func (b *pibuffer) get() <-chan poolItem {
	return b.outch
}

func (b *pibuffer) put(c poolItem) {
	b.inch <- c
}

func (b *pibuffer) stop() {
	close(b.inch)
}

func (b *pibuffer) run() {
	defer close(b.outch)
	for {

		b.uie.EmitNumReady(len(b.buf))

		var next poolItem
		var out chan poolItem

		if len(b.buf) > 0 {
			next = b.buf[0]
			out = b.outch
		}

		select {
		case c, ok := <-b.inch:
			if !ok {
				b.uie.EmitNumReady(0)
				return
			}
			b.buf = append(b.buf, c)
		case out <- next:
			b.buf = b.buf[1:]
		}
	}
}
