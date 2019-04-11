package pubsub

import (
	"time"

	"github.com/boz/ephemerald/types"
	"github.com/boz/go-lifecycle"
)

const (
	bufSiz  = 32
	bufWait = time.Millisecond
)

type Subscription interface {
	Events() <-chan types.BusEvent
	Close()
	Done() <-chan struct{}
}

type subscription struct {
	inch  chan types.BusEvent
	outch chan types.BusEvent

	lc lifecycle.Lifecycle
}

func newSubscription(donech chan<- *subscription, filter Filter) *subscription {

	s := &subscription{
		inch:  make(chan types.BusEvent, bufSiz),
		outch: make(chan types.BusEvent, bufSiz),
	}

	go s.run(donech, filter)

	return s
}

func (s *subscription) Events() <-chan types.BusEvent {
	return s.outch
}

func (s *subscription) Close() {
	s.lc.ShutdownAsync(nil)
}

func (s *subscription) Done() <-chan struct{} {
	return s.lc.Done()
}

func (s *subscription) publish(ev types.BusEvent) {
	select {
	case s.inch <- ev:
	case <-s.lc.ShuttingDown():
	}
}

func (s *subscription) run(donech chan<- *subscription, filter Filter) {
	defer s.lc.ShutdownCompleted()
	defer func() { donech <- s }()

loop:
	for {
		select {

		case err := <-s.lc.ShutdownRequest():
			s.lc.ShutdownInitiated(err)
			break loop

		case ev := <-s.inch:

			if filter != nil && !filter(ev) {
				continue loop
			}

			select {
			case s.outch <- ev:
			case <-time.After(bufWait):
				// TODO
				panic("XXX clean this up")
			}

		}
	}

}
