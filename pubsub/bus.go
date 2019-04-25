package pubsub

import (
	"context"
	"errors"

	"github.com/boz/ephemerald/types"
	"github.com/boz/go-lifecycle"
)

type Writer interface {
	Publish(types.BusEvent) error
}

type Reader interface {
	Subscribe(Filter) (Subscription, error)
}

type Bus interface {
	Writer
	Reader
}

type Service interface {
	Bus
	Shutdown() error
}

func NewBus(ctx context.Context) (Service, error) {

	b := &bus{
		pubch:       make(chan types.BusEvent),
		subch:       make(chan subreq),
		subdonech:   make(chan *subscription),
		subscribers: make(map[*subscription]bool),
		ctx:         ctx,
		lc:          lifecycle.New(),
	}

	go b.lc.WatchContext(ctx)
	go b.run()

	return b, nil
}

type bus struct {
	pubch       chan types.BusEvent
	subch       chan subreq
	subdonech   chan *subscription
	subscribers map[*subscription]bool
	ctx         context.Context
	lc          lifecycle.Lifecycle
}

type subreq struct {
	filter Filter
	ch     chan<- Subscription
}

func (b *bus) Publish(ev types.BusEvent) error {
	select {
	case b.pubch <- ev:
		return nil
	case <-b.lc.ShuttingDown():
		return errors.New("Not Running")
	}
}

func (b *bus) Subscribe(filter Filter) (Subscription, error) {
	ch := make(chan Subscription, 1)
	req := subreq{filter, ch}

	select {
	case b.subch <- req:
	case <-b.lc.ShuttingDown():
		return nil, errors.New("Not Running")
	}

	select {
	case sub := <-ch:
		return sub, nil
	case <-b.lc.ShuttingDown():
		return nil, errors.New("Not Running")
	}
}

func (b *bus) Shutdown() error {
	b.lc.Shutdown(nil)
	return b.lc.Error()
}

func (b *bus) run() {
	defer b.lc.ShutdownCompleted()

loop:
	for {
		select {

		case err := <-b.lc.ShutdownRequest():
			b.lc.ShutdownInitiated(err)
			break loop

		case ev := <-b.pubch:

			for sub := range b.subscribers {
				sub.publish(ev)
			}

		case req := <-b.subch:

			sub := newSubscription(b.subdonech, req.filter)
			b.subscribers[sub] = true
			req.ch <- sub

		case sub := <-b.subdonech:

			delete(b.subscribers, sub)

		}
	}

	for sub := range b.subscribers {
		sub.Close()
	}

	for len(b.subscribers) > 0 {
		delete(b.subscribers, <-b.subdonech)
	}
}
