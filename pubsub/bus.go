package pubsub

import (
	"context"
	"errors"

	"github.com/boz/go-lifecycle"
)

type Bus interface {
	Publish(interface{}) error
	Subscribe(func(interface{}) bool) (Subscription, error)
	Shutdown() error
}

func NewBus(ctx context.Context) (Bus, error) {

	b := &bus{
		pubch:       make(chan interface{}),
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
	pubch       chan interface{}
	subch       chan subreq
	subdonech   chan *subscription
	subscribers map[*subscription]bool
	ctx         context.Context
	lc          lifecycle.Lifecycle
}

type subreq struct {
	filter func(interface{}) bool
	ch     chan<- Subscription
}

func (b *bus) Publish(ev interface{}) error {
	select {
	case b.pubch <- ev:
		return nil
	case <-b.lc.ShuttingDown():
		return errors.New("Not Running")
	}
}

func (b *bus) Subscribe(filter func(interface{}) bool) (Subscription, error) {
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
