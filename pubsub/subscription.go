package pubsub

import (
	"time"

	"github.com/boz/go-lifecycle"
)

const (
	bufSiz  = 32
	bufWait = time.Millisecond
)

type Subscription interface {
	Events() <-chan interface{}
	Close()
	Done() <-chan struct{}
}

type subscription struct {
	inch  chan interface{}
	outch chan interface{}

	lc lifecycle.Lifecycle
}

func newSubscription(donech chan<- *subscription, filter func(interface{}) bool) *subscription {

	s := &subscription{
		inch:  make(chan interface{}, bufSiz),
		outch: make(chan interface{}, bufSiz),
	}

	go s.run(donech, filter)

	return s
}

func (s *subscription) Events() <-chan interface{} {
	return s.outch
}

func (s *subscription) Close() {
	s.lc.ShutdownAsync(nil)
}

func (s *subscription) Done() <-chan struct{} {
	return s.lc.Done()
}

func (s *subscription) publish(ev interface{}) {
	select {
	case s.inch <- ev:
	case <-s.lc.ShuttingDown():
	}
}

func (s *subscription) run(donech chan<- *subscription, filter func(interface{}) bool) {
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
				panic("XXX clean this up")
			}

		}
	}

}
