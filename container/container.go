package container

import (
	"context"
	"errors"

	"github.com/boz/ephemerald/node"
	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/pubsub"
	"github.com/boz/ephemerald/runner"
	"github.com/boz/ephemerald/types"
	"github.com/boz/go-lifecycle"
	dtypes "github.com/docker/docker/api/types"
	dcontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

type Container interface {
	ID() types.ID
	PoolID() types.ID
	Checkout(context.Context) (params.Params, error)
	Release(context.Context) error
	Shutdown()
	Done() <-chan struct{}
}

type Info struct {
	Host string
	Port string
}

type Config struct{}

func Create(bus pubsub.Bus, node node.Node, pid types.ID, config Config) (Container, error) {

	id, err := types.NewID()
	if err != nil {
		return nil, err
	}

	c := &container{
		bus:    bus,
		id:     id,
		pid:    pid,
		node:   node,
		config: config,
		lc:     lifecycle.New(),
	}

	go c.run()

	return c, nil
}

type container struct {
	node   node.Node
	id     types.ID
	pid    types.ID
	config Config

	bus pubsub.Bus

	lc lifecycle.Lifecycle
}

func (c *container) ID() types.ID {
	return c.id
}

func (c *container) PoolID() types.ID {
	return c.pid
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

	_, err := c.create()
	if err != nil {
		c.lc.ShutdownInitiated(err)
		return
	}

loop:
	for {
		select {
		case err := <-c.lc.ShutdownRequest():
			c.lc.ShutdownInitiated(err)
			break loop
		}
	}
}

func (c *container) create() (string, error) {
	ctx, cancel := context.WithCancel(context.Background())

	runch := runner.Do(func() runner.Result {
		return runner.NewResult(c.doCreate(ctx))
	})

	select {
	case err := <-c.lc.ShutdownRequest():
		cancel()
		<-runch
		return "", err
	case res := <-runch:
		cancel()
		if res.Err() != nil {
			return "", res.Err()
		}
		return res.Value().(string), nil
	}
}

func (c *container) doCreate(ctx context.Context) (dtypes.ContainerJSON, error) {
	cconfig := &dcontainer.Config{
		Labels: map[string]string{
			node.LabelEphemeraldPoolID:      string(c.pid),
			node.LabelEphemeraldContainerID: string(c.id),
		},
		/*
			Image:        a.ref.Name(),
			Cmd:          a.config.Container.Cmd,
			Env:          a.config.Container.Env,
			Volumes:      a.config.Container.Volumes,
			Labels:       a.config.Container.Labels,
			AttachStdin:  false,
			AttachStdout: false,
			AttachStderr: false,
			ExposedPorts: nat.PortSet{
				nat.Port(strconv.Itoa(a.config.Port)): struct{}{},
			},
		*/
	}
	hconfig := &dcontainer.HostConfig{
		/*
			AutoRemove:      true,
			PublishAllPorts: true,
			RestartPolicy:   dcontainer.RestartPolicy{},
		*/
	}
	nconfig := &network.NetworkingConfig{}

	cinfo, err := c.node.Client().ContainerCreate(ctx, cconfig, hconfig, nconfig, "")
	if err != nil {
		return dtypes.ContainerJSON{}, err
	}

	if err := c.node.Client().ContainerStart(ctx, cinfo.ID, dtypes.ContainerStartOptions{}); err != nil {
		return dtypes.ContainerJSON{}, err
	}

	info, err := c.node.Client().ContainerInspect(ctx, cinfo.ID)
	if err != nil {
		return info, err
	}

	return info, nil
}
