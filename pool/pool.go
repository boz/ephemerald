package pool

import (
	"context"
	"errors"

	"github.com/boz/ephemerald/params"
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
	return &pool{
		readych: make(chan struct{}),
		ctx:     ctx,
		lc:      lifecycle.New(),
	}
}

type pool struct {
	readych chan struct{}
	ctx     context.Context
	lc      lifecycle.Lifecycle
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
