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

	emitter ui.PoolEmitter
}

func newPoolItemBuffer(emitter ui.PoolEmitter) poolItemBuffer {
	b := &pibuffer{
		outch:   make(chan poolItem),
		inch:    make(chan poolItem),
		emitter: emitter,
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

		b.emitter.EmitNumReady(len(b.buf))

		if len(b.buf) == 0 {
			select {
			case c, ok := <-b.inch:
				if !ok {
					b.emitter.EmitNumReady(0)
					return
				}
				b.buf = append(b.buf, c)
			}
			continue
		}

		next := b.buf[0]
		select {
		case c, ok := <-b.inch:
			if !ok {
				b.emitter.EmitNumReady(0)
				return
			}
			b.buf = append(b.buf, c)
		case b.outch <- next:
			b.buf = b.buf[1:]
		}
	}
}
