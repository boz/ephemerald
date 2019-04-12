package pool

import (
	"context"
	"errors"

	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/instance"
	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/pubsub"
	"github.com/boz/ephemerald/runner"
	"github.com/boz/ephemerald/scheduler"
	"github.com/boz/ephemerald/types"
	"github.com/boz/go-lifecycle"
	"github.com/docker/distribution/reference"
)

type Pool interface {
	Ready() <-chan struct{}

	Checkout(context.Context) (params.Params, error)
	Release(context.Context, types.ID) error

	Shutdown()
	Done() <-chan struct{}
}

func Create(ctx context.Context, bus pubsub.Bus, scheduler scheduler.Scheduler, config config.Pool) (Pool, error) {

	id, err := types.NewID()
	if err != nil {
		return nil, err
	}

	p := &pool{
		id:        id,
		bus:       bus,
		scheduler: scheduler,
		config:    config,

		instances: make(map[types.ID]instance.Instance),
		iready:    make(map[types.ID]instance.Instance),

		checkoutch: make(chan checkoutReq),
		releasech:  make(chan types.ID),
		readych:    make(chan struct{}),

		ctx: ctx,
		lc:  lifecycle.New(),
	}

	go p.lc.WatchContext(ctx)
	go p.run()

	return p, nil
}

type pool struct {
	bus       pubsub.Bus
	id        types.ID
	config    config.Pool
	scheduler scheduler.Scheduler

	instances map[types.ID]instance.Instance
	iready    map[types.ID]instance.Instance

	crequests  []checkoutReq
	checkoutch chan checkoutReq
	releasech  chan types.ID

	readych chan struct{}
	ctx     context.Context
	lc      lifecycle.Lifecycle
}

func (p *pool) Ready() <-chan struct{} {
	return p.readych
}

type checkoutReq struct {
	ch  chan<- params.Params
	ctx context.Context
}

func (p *pool) Checkout(ctx context.Context) (params.Params, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch := make(chan params.Params)

	req := checkoutReq{ch: ch, ctx: ctx}

	select {
	case <-ctx.Done():
		return params.Params{}, ctx.Err()
	case <-p.lc.ShuttingDown():
		return params.Params{}, errors.New("not running")
	case p.checkoutch <- req:
	}

	select {
	case <-ctx.Done():
		return params.Params{}, ctx.Err()
	case <-p.lc.ShuttingDown():
		return params.Params{}, errors.New("not running")
	case val := <-ch:
		return val, nil
	}
}

func (p *pool) Release(ctx context.Context, id types.ID) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.lc.ShuttingDown():
		return errors.New("not running")
	case p.releasech <- id:
		return nil
	}
}

func (p *pool) Shutdown() {
	p.lc.ShutdownAsync(nil)
}

func (p *pool) Done() <-chan struct{} {
	return p.lc.Done()
}

func (p *pool) run() {
	defer p.lc.ShutdownCompleted()

	numStarted := 0

	sub, err := p.subscribe()
	if err != nil {
		p.lc.ShutdownInitiated(err)
		return
	}

	_, err = p.resolveImage()
	if err != nil {
		p.lc.ShutdownInitiated(err)
		goto done
	}

loop:
	for {

		p.fill()

		select {
		case err := <-p.lc.ShutdownRequest():
			p.lc.ShutdownInitiated(err)
			break loop

		case ev := <-sub.Events():

			switch ev.GetAction() {
			case types.EventActionReady:

				instance, ok := p.instances[ev.GetInstance()]
				if !ok {
					// warn
					continue loop
				}

				p.iready[instance.ID()] = instance

				if numStarted == 0 {
					close(p.readych)
				}
				numStarted++

				p.fulfillRequests()

			case types.EventActionDone:

				delete(p.instances, ev.GetInstance())
				delete(p.iready, ev.GetInstance())

			}

		case req := <-p.checkoutch:

			p.crequests = append(p.crequests, req)
			p.fulfillRequests()

		case id := <-p.releasech:
			instance, ok := p.iready[id]
			if !ok {
				// warn
				continue loop
			}
			delete(p.iready, id)
			if err := instance.Release(p.ctx); err != nil {
				// warn
			}
		}
	}

done:

	sub.Close()

	// drain
	for _, instance := range p.instances {
		instance.Shutdown()
	}

	for _, instance := range p.instances {
		<-instance.Done()
	}

	<-sub.Done()
}

func (p *pool) subscribe() (pubsub.Subscription, error) {
	filter := func(ev types.BusEvent) bool {
		return ev.GetPool() == p.id &&
			ev.GetType() == types.EventTypeInstance
	}
	return p.bus.Subscribe(filter)
}

func (p *pool) resolveImage() (reference.Canonical, error) {
	ctx, cancel := context.WithCancel(p.ctx)

	refch := runner.Do(func() runner.Result {
		return runner.NewResult(p.scheduler.ResolveImage(ctx, p.config.Image))
	})

	select {
	case err := <-p.lc.ShutdownRequest():
		cancel()
		<-refch
		return nil, err
	case result := <-refch:
		cancel()
		if result.Err() != nil {
			return nil, result.Err()
		}
		return result.Value().(reference.Canonical), nil
	}

}

func (p *pool) fill() error {
	for len(p.instances) < p.config.Size {
		instance, err := p.scheduler.CreateInstance(p.ctx, p.id, p.config.Container, p.config.Actions)
		if err != nil {
			return err
		}
		p.instances[instance.ID()] = instance
	}
	return nil
}

func (p *pool) fulfillRequests() {

	// clear out stale requests
	for idx, req := range p.crequests {
		if req.ctx.Err() != nil {
			// warn stale request
			p.crequests = append(p.crequests[:idx], p.crequests[idx+1:]...)
		}
	}

loop:
	for len(p.crequests) > 0 && len(p.iready) > 0 {

		for id, instance := range p.iready {
			delete(p.iready, id)

			params, err := instance.Checkout(p.ctx)
			if err != nil {
				// warn
				continue
			}

			for idx, req := range p.crequests {
				p.crequests = append(p.crequests[:idx], p.crequests[idx+1:]...)

				select {
				case <-req.ctx.Done():
					// warn stale request

				case req.ch <- params:
					continue loop
				}
			}

			// XXX: no more requests to check out.
			if err := instance.Release(p.ctx); err != nil {
				// warn.
			}
		}
	}
}
