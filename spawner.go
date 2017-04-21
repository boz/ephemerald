package ephemerald

import (
	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald/lifecycle"
	"github.com/boz/ephemerald/ui"
)

type poolItemSpawner interface {
	stop()
	next() <-chan poolItem
	request(int)
}

type pispawner struct {
	adapter   dockerAdapter
	lifecycle lifecycle.Manager

	pending int
	needed  int

	// close to commence shutdown
	shutdownch chan bool

	// request more children
	requestch chan int

	// results of spawn.
	resultch chan spawnresult

	// where to send new children
	nextch chan poolItem

	log logrus.FieldLogger

	emitter ui.PoolEmitter
}

type spawnresult struct {
	item poolItem
	err  error
}

func newPoolItemSpawner(emitter ui.PoolEmitter, adapter dockerAdapter, lifecycle lifecycle.Manager) poolItemSpawner {
	s := &pispawner{
		adapter:   adapter,
		lifecycle: lifecycle,
		requestch: make(chan int, 5),
		resultch:  make(chan spawnresult),
		nextch:    make(chan poolItem),
		log:       adapter.logger().WithField("component", "spawner"),
		emitter:   emitter,
	}

	go s.run()

	return s
}

func (s *pispawner) stop() {
	close(s.requestch)
}

func (s *pispawner) next() <-chan poolItem {
	return s.nextch
}

func (s *pispawner) request(count int) {
	s.requestch <- count
}

func (s *pispawner) run() {
	s.fillRequests()
	s.drain()
}

func (s *pispawner) fillRequests() {
	for {
		s.fill()

		s.emitter.EmitNumPending(s.pending)

		select {
		case request, ok := <-s.requestch:
			if !ok {
				return
			}
			s.needed = request

		case result := <-s.resultch:
			s.pending--

			if result.err != nil {
				s.log.WithError(result.err).Error("can't create child")
				continue
			}

			if s.needed > 0 {
				s.needed--
			}

			s.nextch <- result.item
		}
	}
}

func (s *pispawner) drain() {
	defer close(s.nextch)

	for {
		s.emitter.EmitNumPending(s.pending)

		if s.pending <= 0 {
			return
		}

		select {
		case result := <-s.resultch:
			s.pending--

			if result.err != nil {
				s.log.WithError(result.err).Error("can't create child")
				continue
			}
			s.nextch <- result.item
		}
	}
}

func (s *pispawner) fill() {
	for ; s.pending < s.needed; s.pending++ {
		go func() {
			item, err := createPoolItem(s.emitter, s.log, s.adapter, s.lifecycle)
			s.resultch <- spawnresult{item, err}
		}()
	}
}
