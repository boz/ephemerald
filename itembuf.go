package ephemerald

type itembuf struct {
	outch chan PoolItem
	inch  chan PoolItem
	buf   []PoolItem
}

func newItemBuffer() *itembuf {
	b := &itembuf{
		outch: make(chan PoolItem),
		inch:  make(chan PoolItem),
	}
	go b.run()
	return b
}

func (b *itembuf) Get() <-chan PoolItem {
	return b.outch
}

func (b *itembuf) Put(c PoolItem) {
	b.inch <- c
}

func (b *itembuf) Stop() {
	close(b.inch)
}

func (b *itembuf) run() {
	defer close(b.outch)
	for {

		if len(b.buf) == 0 {
			select {
			case c, ok := <-b.inch:
				if !ok {
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
				return
			}
			b.buf = append(b.buf, c)
		case b.outch <- next:
			b.buf = b.buf[1:]
		}
	}
}
