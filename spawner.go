package ephemerald

import (
	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald/lifecycle"
)

type spawner struct {
	adapter   Adapter
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
	nextch chan PoolItem

	log logrus.FieldLogger
}

type spawnresult struct {
	item PoolItem
	err  error
}

func newSpawner(adapter Adapter, lifecycle lifecycle.Manager) *spawner {
	s := &spawner{
		adapter:   adapter,
		lifecycle: lifecycle,
		requestch: make(chan int, 5),
		resultch:  make(chan spawnresult),
		nextch:    make(chan PoolItem),
		log:       adapter.Log().WithField("component", "spawner"),
	}

	go s.run()

	return s
}

func (s *spawner) Stop() {
	close(s.requestch)
}

func (s *spawner) Next() <-chan PoolItem {
	return s.nextch
}

func (s *spawner) Request(count int) {
	s.requestch <- count
}

func (s *spawner) run() {
	s.fillRequests()
	s.drain()
}

func (s *spawner) fillRequests() {
	for {
		s.fill()
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

func (s *spawner) drain() {
	defer close(s.nextch)

	for s.pending > 0 {
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

func (s *spawner) fill() {
	for ; s.pending < s.needed; s.pending++ {
		go func() {
			item, err := createPoolItem(s.log, s.adapter, s.lifecycle)
			s.resultch <- spawnresult{item, err}
		}()
	}
}
