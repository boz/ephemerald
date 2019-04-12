package instance

import (
	"context"
	"errors"

	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/lifecycle"
	"github.com/boz/ephemerald/node"
	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/pubsub"
	"github.com/boz/ephemerald/runner"
	"github.com/boz/ephemerald/types"
	golifecycle "github.com/boz/go-lifecycle"
	dtypes "github.com/docker/docker/api/types"
	dcontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

type iState string

const (
	iStateCreate      iState = "create"
	iStateStart              = "start"
	iStateInitialize         = "initialize"
	iStateHealthcheck        = "healthcheck"
	iStateReady              = "ready"
	iStateCheckout           = "checkout"
	iStateReset              = "reset"
	iStateKill               = "kill"
	iStateDone               = "done"
)

type Instance interface {
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

func Create(bus pubsub.Bus, node node.Node, pid types.ID, cconfig config.Container, lifecycle lifecycle.Manager) (Instance, error) {

	id, err := types.NewID()
	if err != nil {
		return nil, err
	}

	i := &instance{
		state:      iStateCreate,
		bus:        bus,
		id:         id,
		pid:        pid,
		node:       node,
		cconfig:    cconfig,
		checkoutch: make(chan checkoutReq),
		releasech:  make(chan chan<- error),
		lifecycle:  lifecycle,
		lc:         golifecycle.New(),
	}

	go i.run()

	return i, nil
}

type instance struct {
	state   iState
	node    node.Node
	id      types.ID
	pid     types.ID
	cconfig config.Container

	checkoutch chan checkoutReq
	releasech  chan chan<- error

	lifecycle lifecycle.Manager
	bus       pubsub.Bus

	lc golifecycle.Lifecycle
}

func (i *instance) ID() types.ID {
	return i.id
}

func (i *instance) PoolID() types.ID {
	return i.pid
}

type checkoutReq struct {
	pch chan<- params.Params
	ech chan<- error
	ctx context.Context
}

func (i *instance) Checkout(ctx context.Context) (params.Params, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pch := make(chan params.Params)
	ech := make(chan error)
	req := checkoutReq{pch: pch, ech: ech, ctx: ctx}

	select {
	case <-ctx.Done():
		return params.Params{}, ctx.Err()
	case <-i.lc.ShuttingDown():
		return params.Params{}, errors.New("not running")
	case i.checkoutch <- req:
	}

	select {
	case <-ctx.Done():
		return params.Params{}, ctx.Err()
	case <-i.lc.ShuttingDown():
		return params.Params{}, errors.New("not running")
	case err := <-ech:
		return params.Params{}, err
	case params := <-pch:
		return params, nil
	}
}

func (i *instance) Release(ctx context.Context) error {
	ch := make(chan error, 1)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-i.lc.ShuttingDown():
		return errors.New("not running")
	case i.releasech <- ch:
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-i.lc.ShuttingDown():
		return errors.New("not running")
	case err := <-ch:
		return err
	}
}

func (i *instance) Shutdown() {
	i.lc.ShutdownAsync(nil)
}

func (i *instance) Done() <-chan struct{} {
	return i.lc.Done()
}

func (i *instance) run() {
	defer i.lc.ShutdownCompleted()

	var iparams params.Params

	sub, err := i.subscribe()
	if err != nil {
		i.lc.ShutdownInitiated(err)
		return
	}

	cinfo, err := i.create()
	if err != nil {
		i.lc.ShutdownInitiated(err)
		goto kill
	}

	iparams, err = params.ParamsFor(types.Instance{
		ID:     i.id,
		PoolID: i.pid,
		Host:   i.node.Endpoint(),
	}, cinfo, 80)

	if err != nil {
		i.lc.ShutdownInitiated(err)
		goto kill
	}

	i.state = iStateInitialize

loop:
	for {
		select {
		case err := <-i.lc.ShutdownRequest():
			i.lc.ShutdownInitiated(err)
			break loop

		case req := <-i.checkoutch:

			if i.state != iStateReady {
				req.ech <- errors.New("invalid state")
				continue loop
			}

			select {
			case <-req.ctx.Done():
				// warn error
				continue loop
			case req.pch <- iparams:
			}

			// todo: run lifecycle

		case req := <-i.releasech:
			if i.state != iStateCheckout {
				req <- errors.New("invalid state")
				continue loop
			}
			req <- nil

			// todo: run lifecycle.
		}
	}

kill:

	sub.Close()
	<-sub.Done()
}

func (i *instance) subscribe() (pubsub.Subscription, error) {
	filter := func(ev types.BusEvent) bool {
		return ev.GetInstance() == i.id &&
			ev.GetPool() == i.pid &&
			ev.GetType() != types.EventTypeInstance
	}
	return i.bus.Subscribe(filter)
}

func (i *instance) create() (dtypes.ContainerJSON, error) {
	ctx, cancel := context.WithCancel(context.Background())

	runch := runner.Do(func() runner.Result {
		return runner.NewResult(i.doCreate(ctx))
	})

	select {
	case err := <-i.lc.ShutdownRequest():
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

func (i *instance) doCreate(ctx context.Context) (dtypes.ContainerJSON, error) {
	cconfig := &dcontainer.Config{
		Labels: map[string]string{
			node.LabelEphemeraldPoolID:      string(i.pid),
			node.LabelEphemeraldContainerID: string(i.id),
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

	cinfo, err := i.node.Client().ContainerCreate(ctx, cconfig, hconfig, nconfig, "")
	if err != nil {
		return dtypes.ContainerJSON{}, err
	}

	// emit created, state start

	if err := i.node.Client().ContainerStart(ctx, cinfo.ID, dtypes.ContainerStartOptions{}); err != nil {
		return dtypes.ContainerJSON{}, err
	}

	info, err := i.node.Client().ContainerInspect(ctx, cinfo.ID)
	if err != nil {
		return info, err
	}

	// emit started, state initialize

	return info, nil
}
