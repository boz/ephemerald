package pool

import (
	"context"
	"errors"

	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/runner"
	"github.com/boz/ephemerald/scheduler"
	"github.com/boz/ephemerald/types"
	"github.com/boz/go-lifecycle"
	"github.com/docker/distribution/reference"
)

type Pool interface {
	Ready() <-chan struct{}

	Checkout(context.Context) (params.Params, error)
	Release(context.Context, string) error

	Shutdown()
	Done() <-chan struct{}
}

func Create(ctx context.Context) (Pool, error) {

	id, err := types.NewID()
	if err != nil {
		return nil, err
	}

	p := &pool{
		id:      id,
		readych: make(chan struct{}),
		config:  config{},
		ctx:     ctx,
		lc:      lifecycle.New(),
	}

	go p.lc.WatchContext(ctx)
	go p.run()

	return p, nil
}

type pool struct {
	id        types.PoolID
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

	_, err := p.resolveImage()
	if err != nil {
		p.lc.ShutdownInitiated(err)
		return
	}

loop:
	for {

		// create containers
		// for i := len(containers); i < 10; i++ {
		// }
		// for len(containers) < 10 {
		// }

		select {
		case err := <-p.lc.ShutdownRequest():
			p.lc.ShutdownInitiated(err)
			break loop

		case cid := <-creadych:
			// container ready

		case cid := <-cdonech:
			// container done

		case req := <-ccheckoutch:
			// container checkout

		case cid := <-creleasech:
			// container release
		}
	}
}

func (p *pool) resolveImage() (reference.Canonical, error) {
	ctx, cancel := context.WithCancel(p.ctx)

	refch := runner.Do(func() runner.Result {
		return runner.NewResult(p.scheduler.ResolveImage(ctx, p.config.imageName))
	})

	select {
	case err := <-p.lc.ShutdownRequest():
		cancel()
		<-refch
		return nil, err
	case result := <-refch:
		if result.Err() != nil {
			return nil, result.Err()
		}
		return result.Value().(reference.Canonical), nil
	}

}
