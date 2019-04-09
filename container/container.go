package container

import (
	"context"
	"errors"

	"github.com/boz/ephemerald/node"
	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/types"
	"github.com/boz/go-lifecycle"
)

type Container interface {
	ID() types.ContainerID
	Checkout(context.Context) (params.Params, error)
	Release(context.Context) error
	Shutdown()
	Done() <-chan struct{}
}

type Config struct{}

func Create(node node.Node, pid types.PoolID, config Config) (Container, error) {

	id, err := types.NewID()
	if err != nil {
		return nil, err
	}

	c := &container{
		id:     types.ContainerID{id, pid},
		node:   node,
		config: config,
		lc:     lifecycle.New(),
	}

	go c.run()

	return c, nil
}

type container struct {
	node   node.Node
	id     types.ContainerID
	config Config

	lc lifecycle.Lifecycle
}

func (c container) ID() types.ContainerID {
	return c.id
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
