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

type containerState string

const (
	containerStateStart       containerState = "start"
	containerStateInitialize                 = "initialize"
	containerStateHealthcheck                = "healthcheck"
	containerStateReady                      = "ready"
	containerStateCheckout                   = "checkout"
	containerStateReset                      = "reset"
	containerStateKill                       = "kill"
	containerStateDone                       = "done"
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
		state:  containerStateStart,
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
	state  containerState
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

	cinfo, err := c.create()
	if err != nil {
		c.lc.ShutdownInitiated(err)
		return
	}

	_, err = params.ParamsFor(types.Container{
		ID:     c.id,
		PoolID: c.pid,
		Host:   c.node.Endpoint(),
	}, cinfo, 80)

	if err != nil {
		c.lc.ShutdownInitiated(err)
		goto kill
	}

	c.state = containerStateInitialize

loop:
	for {
		select {
		case err := <-c.lc.ShutdownRequest():
			c.lc.ShutdownInitiated(err)
			break loop
		}
	}

kill:
}

func (c *container) create() (dtypes.ContainerJSON, error) {
	ctx, cancel := context.WithCancel(context.Background())

	runch := runner.Do(func() runner.Result {
		return runner.NewResult(c.doCreate(ctx))
	})

	select {
	case err := <-c.lc.ShutdownRequest():
		cancel()
		<-runch
		return dtypes.ContainerJSON{}, err
	case res := <-runch:
		cancel()
		if res.Err() != nil {
			return dtypes.ContainerJSON{}, res.Err()
		}
		return res.Value().(dtypes.ContainerJSON), nil
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
