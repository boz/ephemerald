package poolset

import (
	"context"
	"errors"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/pool"
	"github.com/boz/ephemerald/pubsub"
	"github.com/boz/ephemerald/scheduler"
	"github.com/boz/ephemerald/types"
	"github.com/boz/go-lifecycle"
)

type PoolSet interface {
	Create(context.Context, config.Pool) (pool.Pool, error)
	Get(context.Context, types.ID) (pool.Pool, error)
	List(context.Context, types.ID) ([]pool.Pool, error)
	Delete(context.Context, types.ID) error
}

type Service interface {
	PoolSet
	Shutdown()
	Done() <-chan struct{}
}

func New(ctx context.Context, bus pubsub.Bus, scheduler scheduler.Scheduler) (Service, error) {

	pset := &poolset{
		bus:       bus,
		scheduler: scheduler,
		pools:     make(map[types.ID]pool.Pool),

		cch: make(chan creq),
		gch: make(chan greq),
		lch: make(chan lreq),
		dch: make(chan dreq),

		ctx: ctx,
		lc:  lifecycle.New(),
	}

	go pset.lc.WatchContext(ctx)
	go pset.run()

	return nil, nil
}

type poolset struct {
	bus       pubsub.Bus
	scheduler scheduler.Scheduler

	pools map[types.ID]pool.Pool

	cch chan creq
	gch chan greq
	lch chan lreq
	dch chan dreq

	ctx context.Context
	lc  lifecycle.Lifecycle
	l   logrus.FieldLogger
}

type creq struct {
	ctx context.Context
	cfg config.Pool
	ch  chan<- pool.Pool
	ech chan<- error
}

func (pset *poolset) Create(ctx context.Context, cfg config.Pool) (pool.Pool, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ech := make(chan error, 1)
	ch := make(chan pool.Pool, 1)

	req := creq{
		ctx: ctx,
		cfg: cfg,
		ch:  ch,
		ech: ech,
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-pset.lc.ShuttingDown():
		return nil, errors.New("not running")
	case pset.cch <- req:
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-pset.lc.ShuttingDown():
		return nil, errors.New("not running")
	case err := <-ech:
		return nil, err
	case pool := <-ch:
		return pool, nil
	}
}

type greq struct {
	ctx context.Context
	id  types.ID
	ch  chan<- pool.Pool
	ech chan<- error
}

func (pset *poolset) Get(ctx context.Context, id types.ID) (pool.Pool, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ech := make(chan error, 1)
	ch := make(chan pool.Pool, 1)

	req := greq{
		ctx: ctx,
		id:  id,
		ch:  ch,
		ech: ech,
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-pset.lc.ShuttingDown():
		return nil, errors.New("not running")
	case pset.gch <- req:
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-pset.lc.ShuttingDown():
		return nil, errors.New("not running")
	case err := <-ech:
		return nil, err
	case pool := <-ch:
		return pool, nil
	}
}

type lreq struct {
	ctx context.Context
	ch  chan<- []pool.Pool
	ech chan<- error
}

func (pset *poolset) List(ctx context.Context) ([]pool.Pool, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ech := make(chan error, 1)
	ch := make(chan []pool.Pool, 1)

	req := lreq{
		ctx: ctx,
		ch:  ch,
		ech: ech,
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-pset.lc.ShuttingDown():
		return nil, errors.New("not running")
	case pset.lch <- req:
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-pset.lc.ShuttingDown():
		return nil, errors.New("not running")
	case err := <-ech:
		return nil, err
	case val := <-ch:
		return val, nil
	}
}

type dreq struct {
	ctx context.Context
	id  types.ID
	ech chan<- error
}

func (pset *poolset) Delete(ctx context.Context, id types.ID) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ech := make(chan error, 1)

	req := dreq{
		ctx: ctx,
		id:  id,
		ech: ech,
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-pset.lc.ShuttingDown():
		return errors.New("not running")
	case pset.dch <- req:
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-pset.lc.ShuttingDown():
		return errors.New("not running")
	case err := <-ech:
		return err
	}
}

func (pset *poolset) run() {
	defer pset.lc.ShutdownCompleted()

loop:
	for {
		select {

		case err := <-pset.lc.ShutdownRequest():
			pset.lc.ShutdownInitiated(err)
			break loop

		case req := <-pset.cch:

			pool, err := pool.Create(pset.ctx, pset.bus, pset.scheduler, req.cfg)
			if err != nil {
				req.ech <- err
			} else {
				pset.pools[pool.ID()] = pool
				req.ch <- pool
			}

		case req := <-pset.gch:

			if pool, ok := pset.pools[req.id]; ok {
				req.ch <- pool
			} else {
				req.ech <- errors.New("not found")
			}

		case req := <-pset.lch:

			objs := make([]pool.Pool, len(pset.pools), 0)
			for _, pool := range pset.pools {
				objs = append(objs, pool)
			}
			req.ch <- objs

		case req := <-pset.dch:

			if pool, ok := pset.pools[req.id]; ok {
				pool.Shutdown()
				req.ech <- nil
			} else {
				req.ech <- errors.New("not found")
			}

		}
	}

	for _, pool := range pset.pools {
		pool.Shutdown()
	}

	for _, pool := range pset.pools {
		<-pool.Done()
	}

}
