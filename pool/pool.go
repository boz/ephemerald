package pool

import (
	"context"
	"errors"

	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/instance"
	"github.com/boz/ephemerald/log"
	"github.com/boz/ephemerald/pubsub"
	"github.com/boz/ephemerald/runner"
	"github.com/boz/ephemerald/scheduler"
	"github.com/boz/ephemerald/types"
	"github.com/boz/go-lifecycle"
	"github.com/docker/distribution/reference"
	"github.com/sirupsen/logrus"
)

type Pool interface {
	ID() types.ID
	Name() string
	Ready() <-chan struct{}

	Checkout(context.Context) (*types.Checkout, error)
	Release(context.Context, types.ID) error
	Model(context.Context) (*types.Pool, error)

	Shutdown()
	Done() <-chan struct{}
}

func Create(ctx context.Context, bus pubsub.Bus, scheduler scheduler.Scheduler, config config.Pool) (Pool, error) {

	id, err := types.NewID()
	if err != nil {
		return nil, err
	}

	l := log.FromContext(ctx).
		WithField("cmp", "pool").
		WithField("pid", id)

	p := &pool{
		id:        id,
		bus:       bus,
		scheduler: scheduler,
		config:    config,

		model: &types.Pool{
			ID:    id,
			Name:  config.Name,
			State: types.PoolStateStart,
			Size:  config.Size,
		},

		instances: make(map[types.ID]instance.Instance),
		iready:    make(map[types.ID]instance.Instance),
		icheckout: make(map[types.ID]instance.Instance),

		checkoutch: make(chan checkoutReq),
		releasech:  make(chan types.ID),
		readych:    make(chan struct{}),
		modelch:    make(chan chan<- *types.Pool),

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
	image     reference.Canonical
	scheduler scheduler.Scheduler
	model     *types.Pool

	instances map[types.ID]instance.Instance
	iready    map[types.ID]instance.Instance
	icheckout map[types.ID]instance.Instance

	crequests  []checkoutReq
	checkoutch chan checkoutReq
	releasech  chan types.ID
	modelch    chan chan<- *types.Pool

	readych chan struct{}
	ctx     context.Context
	lc      lifecycle.Lifecycle

	l logrus.FieldLogger
}

func (p *pool) ID() types.ID {
	return p.id
}

func (p *pool) Name() string {
	return p.config.Name
}

func (p *pool) Ready() <-chan struct{} {
	return p.readych
}

type checkoutReq struct {
	ch  chan<- *types.Checkout
	ctx context.Context
}

func (p *pool) Checkout(ctx context.Context) (*types.Checkout, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch := make(chan *types.Checkout)

	req := checkoutReq{ch: ch, ctx: ctx}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.lc.ShuttingDown():
		return nil, errors.New("not running")
	case p.checkoutch <- req:
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.lc.ShuttingDown():
		return nil, errors.New("not running")
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

func (p *pool) Model(ctx context.Context) (*types.Pool, error) {

	ch := make(chan *types.Pool, 1)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.lc.ShuttingDown():
		return nil, errors.New("not running")
	case p.modelch <- ch:
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.lc.ShuttingDown():
		return nil, errors.New("not running")
	case model := <-ch:
		return model, nil
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

	p.model.State = types.PoolStateResolve
	p.publishStats(types.EventActionStart)

	defer p.publishStats(types.EventActionDone)

	var ok bool
	p.image, ok, err = p.resolveImage()
	if !ok {
		p.lc.ShutdownInitiated(err)
		goto done
	}

	p.model.State = types.PoolStateRun

loop:
	for {

		p.fill()
		p.publishStats(types.EventActionUpdate)

		select {
		case err := <-p.lc.ShutdownRequest():
			p.lc.ShutdownInitiated(err)
			break loop

		case ev := <-sub.Events():

			l := p.l.WithField("ev:action", ev.GetAction()).
				WithField("ev:type", ev.GetType()).
				WithField("ev:iid", ev.GetInstanceID())

			l.Debug("event received")

			switch ev.GetAction() {
			case types.EventActionReady:

				instance, ok := p.instances[ev.GetInstanceID()]
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

				delete(p.instances, ev.GetInstanceID())
				delete(p.iready, ev.GetInstanceID())

			}

		case req := <-p.checkoutch:

			p.crequests = append(p.crequests, req)
			p.fulfillRequests()

		case id := <-p.releasech:

			instance, ok := p.icheckout[id]
			if !ok {
				p.l.WithField("iid", id).Warn("release: unknown instance")
				continue loop
			}

			delete(p.icheckout, id)

			if err := instance.Release(p.ctx); err != nil {
				p.l.WithField("iid", id).
					WithError(err).Warn("release: error releasing")
			}
		case ch := <-p.modelch:
			ch <- &(*p.model)
		}

	}

done:

	p.l.WithField("num-instances", len(p.instances)).Info("shutting down")

	p.model.State = types.PoolStateShutdown
	p.publishStats(types.EventActionShutdown)

	sub.Close()

	// drain
	for _, instance := range p.instances {
		instance.Shutdown()
	}

	for id, instance := range p.instances {
		<-instance.Done()
		delete(p.instances, id)
		delete(p.iready, id)
		delete(p.icheckout, id)
		p.publishStats(types.EventActionUpdate)
	}

	<-sub.Done()
}

func (p *pool) subscribe() (pubsub.Subscription, error) {
	filter := func(ev types.BusEvent) bool {
		return ev.GetPoolID() == p.id &&
			ev.GetType() == types.EventTypeInstance
	}
	sub, err := p.bus.Subscribe(filter)
	if err != nil {
		p.l.WithError(err).Error("bus-subscribe")
	}
	return sub, err
}

func (p *pool) resolveImage() (reference.Canonical, bool, error) {
	ctx, cancel := context.WithCancel(p.ctx)

	refch := runner.Do(func() runner.Result {
		return runner.NewResult(p.scheduler.ResolveImage(ctx, p.config.Image))
	})

	select {
	case err := <-p.lc.ShutdownRequest():
		cancel()
		<-refch
		return nil, false, err
	case result := <-refch:
		cancel()
		if result.Err() != nil {
			p.l.WithError(result.Err()).Error("image-resolve")
			return nil, false, result.Err()
		}
		return result.Value().(reference.Canonical), true, nil
	}

}

func (p *pool) fill() error {

	p.l.WithField("num-instances", p.config.Size-len(p.instances)).Debug("filling")

	for len(p.instances) < p.config.Size {
		instance, err := p.scheduler.CreateInstance(p.ctx, instance.Config{
			Image:   p.image,
			PoolID:  p.id,
			Port:    p.config.Port,
			Actions: p.config.Actions,
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
			p.l.Info("stale request dropped")
			p.crequests = append(p.crequests[:idx], p.crequests[idx+1:]...)
		}
	}

loop:
	for len(p.crequests) > 0 && len(p.iready) > 0 {

		for id, instance := range p.iready {
			delete(p.iready, id)

			co, err := instance.Checkout(p.ctx)
			if err != nil {
				p.l.WithField("iid", id).Warn("checkout failed")
				continue
			}

			for idx, req := range p.crequests {
				p.crequests = append(p.crequests[:idx], p.crequests[idx+1:]...)

				select {
				case <-req.ctx.Done():
					p.l.Warn("stale request")

				case req.ch <- co:
					p.icheckout[id] = instance
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

func (p *pool) publishStats(action types.EventAction) {
	p.updateStats()

	ev := types.Event{
		Type:   types.EventTypePool,
		Action: action,
		Pool:   &(*p.model),
	}

	if action == types.EventActionDone {
		if err := p.lc.Error(); err != nil {
			ev.Status = types.StatusFailure
			ev.Message = err.Error()
		} else {
			ev.Status = types.StatusSuccess
		}
	} else {
		ev.Status = types.StatusInProgress
	}

	if err := p.bus.Publish(ev); err != nil {
		p.l.WithError(err).Error("publish result")
	}
}

func (p *pool) updateStats() {
	p.model.Containers.Total = len(p.instances)
	p.model.Containers.Ready = len(p.iready)
	p.model.Containers.Checkout = len(p.icheckout)
	p.model.Containers.Requests = len(p.crequests)
}
