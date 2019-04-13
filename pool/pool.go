package pool

import (
	"context"
	"errors"

	"github.com/Sirupsen/logrus"
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

	l := logrus.StandardLogger().
		WithField("cmp", "pool").
		WithField("pid", id)

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
		l:   l,
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

	l logrus.FieldLogger
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
	defer func() { p.l.Debug("done") }()
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

			l := p.l.WithField("ev:action", ev.GetAction()).
				WithField("ev:type", ev.GetType()).
				WithField("ev:iid", ev.GetInstance())

			l.Debug("event received")

			switch ev.GetAction() {
			case types.EventActionReady:

				instance, ok := p.instances[ev.GetInstance()]
				if !ok {
					l.Warn("unknown instance")
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
				p.l.WithField("iid", id).Warn("release: unknown instance")
				continue loop
			}

			delete(p.iready, id)

			if err := instance.Release(p.ctx); err != nil {
				p.l.WithField("iid", id).
					WithError(err).Warn("release: error releasing")
			}
		}
	}

done:

	p.l.WithField("num-instances", len(p.instances)).Info("shutting down")

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
	sub, err := p.bus.Subscribe(filter)
	if err != nil {
		p.l.WithError(err).Error("bus-subscribe")
	}
	return sub, err
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
			p.l.WithError(result.Err()).Error("image-resolve")
			return nil, result.Err()
		}
		return result.Value().(reference.Canonical), nil
	}

}

func (p *pool) fill() error {

	p.l.WithField("num-instances", p.config.Size-len(p.instances)).Debug("filling")

	for len(p.instances) < p.config.Size {
		instance, err := p.scheduler.CreateInstance(p.ctx, instance.Config{
			PoolID:    p.id,
			Port:      p.config.Port,
			Container: p.config.Container,
			Actions:   p.config.Actions,
		})
		if err != nil {
			p.l.WithError(err).Error("create-instance")
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
			p.l.Debug("stale request dropped")
			p.crequests = append(p.crequests[:idx], p.crequests[idx+1:]...)
		}
	}

loop:
	for len(p.crequests) > 0 && len(p.iready) > 0 {

		for id, instance := range p.iready {
			delete(p.iready, id)

			params, err := instance.Checkout(p.ctx)
			if err != nil {
				p.l.WithField("iid", id).Warn("checkout failed")
				continue
			}

			for idx, req := range p.crequests {
				p.crequests = append(p.crequests[:idx], p.crequests[idx+1:]...)

				select {
				case <-req.ctx.Done():
					p.l.Warn("stale request")

				case req.ch <- params:
					continue loop
				}
			}

			p.l.WithField("iid", id).Warn("unused checkout found")

			// XXX: no more requests to check out.
			if err := instance.Release(p.ctx); err != nil {
				p.l.WithField("iid", id).WithError(err).Warn("releasing unused checkout")
			}
		}
	}
}
