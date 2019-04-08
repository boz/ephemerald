package pool

import (
	"context"
	"errors"

	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/scheduler"
	"github.com/boz/go-lifecycle"
)

type Pool interface {
	Ready() <-chan struct{}

	Checkout(context.Context) (params.Params, error)
	Release(context.Context, string) error

	Shutdown()
	Done() <-chan struct{}
}

func Create(ctx context.Context) Pool {
	p := &pool{
		readych: make(chan struct{}),
		config:  config{},
		ctx:     ctx,
		lc:      lifecycle.New(),
	}

	go p.lc.WatchContext(ctx)
	go p.run()

	return p
}

type pool struct {
	config    config
	scheduler scheduler.Scheduler
	readych   chan struct{}
	ctx       context.Context
	lc        lifecycle.Lifecycle
}

func (p *pool) Ready() <-chan struct{} {
	return p.readych
}

func (p *pool) Checkout(ctx context.Context) (params.Params, error) {
	return params.Params{}, errors.New("not implemented")
}

func (p *pool) Release(ctx context.Context, id string) error {
	return errors.New("not implemented")
}

func (p *pool) Shutdown() {
	p.lc.ShutdownAsync(nil)
}

func (p *pool) Done() <-chan struct{} {
	return p.lc.Done()
}

func (p *pool) run() {
	defer p.lc.ShutdownCompleted()

	go func() {
		img, err := p.scheduler.ResolveImage(p.ctx, p.config.imageName)
	}()

loop:
	for {
		select {
		case err := <-p.lc.ShutdownRequest():
			p.lc.ShutdownInitiated(err)
			break loop
		}
	}

}
