package container

import (
	"context"
	"errors"

	"github.com/boz/ephemerald/node"
	"github.com/boz/ephemerald/params"
	"github.com/boz/go-lifecycle"
)

/*
PoolID
Node - docker client, endpoint info
EventBus
*/

type Container interface {
	Checkout(context.Context) (params.Params, error)
	Release(context.Context) error
	Shutdown()
	Done() <-chan struct{}
}

type Config struct{}

func Create(node node.Node, poolID string, config Config) (Container, error) {

	c := &container{
		node:   node,
		poolID: poolID,
		config: config,
		lc:     lifecycle.New(),
	}

	go c.run()

	return c, nil
}

type container struct {
	node   node.Node
	poolID string
	config Config

	lc lifecycle.Lifecycle
}

func (c *container) Checkout(ctx context.Context) (params.Params, error) {
	return params.Params{}, errors.New("not implemented")
}

func (c *container) Release(ctx context.Context) error {
	return errors.New("not implemented")
}

func (c *container) Shutdown() {
	c.lc.ShutdownAsync(nil)
}

func (c *container) Done() <-chan struct{} {
	return c.lc.Done()
}

func (c *container) run() {
	defer c.lc.ShutdownCompleted()

loop:
	for {
		select {
		case err := <-c.lc.ShutdownRequest():
			c.lc.ShutdownInitiated(err)
			break loop
		}
	}
}
