package instance

import (
	"context"
	"errors"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/lifecycle"
	"github.com/boz/ephemerald/node"
	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/pubsub"
	"github.com/boz/ephemerald/runner"
	"github.com/boz/ephemerald/types"
	golifecycle "github.com/boz/go-lifecycle"
	"github.com/docker/distribution/reference"
	dtypes "github.com/docker/docker/api/types"
	dcontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
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

type Config struct {
	PoolID    types.ID
	Image     reference.Canonical
	Port      int
	Container config.Container
	Actions   lifecycle.Manager
}

func Create(bus pubsub.Bus, node node.Node, config Config) (Instance, error) {

	id, err := types.NewID()
	if err != nil {
		return nil, err
	}

	l := logrus.StandardLogger().
		WithField("cmp", "instance").
		WithField("pid", config.PoolID).
		WithField("iid", id)

	i := &instance{
		id:         id,
		state:      iStateCreate,
		node:       node,
		config:     config,
		bus:        bus,
		checkoutch: make(chan checkoutReq),
		releasech:  make(chan chan<- error),
		lc:         golifecycle.New(),
		l:          l,
	}

	go i.run()

	return i, nil
}

type instance struct {
	id    types.ID
	state iState

	node   node.Node
	config Config
	bus    pubsub.Bus

	checkoutch chan checkoutReq
	releasech  chan chan<- error

	lc golifecycle.Lifecycle
	l  logrus.FieldLogger
}

func (i *instance) ID() types.ID {
	return i.id
}

func (i *instance) PoolID() types.ID {
	return i.config.PoolID
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

	var (
		iparams params.Params
		cid     string
		cinfo   dtypes.ContainerJSON
		model   types.Instance
		manager lifecycle.ContainerManager
	)

	sub, err := i.subscribe()
	if err != nil {
		i.lc.ShutdownInitiated(err)
		return
	}

	cid, err = i.create()
	if err != nil {
		i.lc.ShutdownInitiated(err)
		goto kill
	}

	cinfo, err = i.start(cid)
	if err != nil {
		i.lc.ShutdownInitiated(err)
		goto kill
	}

	model = types.Instance{
		ID:     i.id,
		PoolID: i.PoolID(),
		Host:   i.node.Endpoint(),
	}

	iparams, err = params.ParamsFor(model, cinfo, 80)

	if err != nil {
		i.lc.ShutdownInitiated(err)
		goto kill
	}

	i.state = iStateInitialize

	manager = i.lifecycle.ForInstance(model)

	actionch = i.runAction(manager.StartCheck)

	// todo: run lifecycle

loop:
	for {
		select {
		case err := <-i.lc.ShutdownRequest():
			i.lc.ShutdownInitiated(err)
			break loop

		case req := <-i.checkoutch:

			if i.state != iStateReady {
				i.l.Warn("checkout: invalid state")
				req.ech <- errors.New("invalid state")
				continue loop
			}

			select {
			case <-req.ctx.Done():
				i.l.Warn("checkout: stale request")
				continue loop
			case req.pch <- iparams:
			}

		case req := <-i.releasech:
			if i.state != iStateCheckout {
				i.l.Warn("release: invalid state")
				req <- errors.New("invalid state")
				continue loop
			}
			req <- nil

			// todo: run lifecycle.
		}
	}

kill:

	sub.Close()
	i.kill(cid)
	<-sub.Done()
}

func (i *instance) subscribe() (pubsub.Subscription, error) {
	filter := func(ev types.BusEvent) bool {
		return ev.GetInstance() == i.id &&
			ev.GetPool() == i.PoolID() &&
			ev.GetType() != types.EventTypeInstance
	}
	sub, err := i.bus.Subscribe(filter)
	if err != nil {
		i.l.WithError(err).Error("bus-subscribe")
	}
	return sub, err
}

func (i *instance) create() (string, error) {
	// todo: timeout
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runch := runner.Do(func() runner.Result {
		return runner.NewResult(i.doCreate(ctx))
	})

	select {
	case err := <-i.lc.ShutdownRequest():
		res := <-runch
		return res.Value().(string), err
	case res := <-runch:
		return res.Value().(string), res.Err()
	}
}

func (i *instance) doCreate(ctx context.Context) (string, error) {
	cconfig := &dcontainer.Config{
		Labels: map[string]string{
			node.LabelEphemeraldPoolID:      string(i.PoolID()),
			node.LabelEphemeraldContainerID: string(i.id),
		},
		Image:        i.config.Image.Name(),
		Cmd:          i.config.Container.Cmd,
		Env:          i.config.Container.Env,
		Volumes:      i.config.Container.Volumes,
		AttachStdin:  false,
		AttachStdout: false,
		AttachStderr: false,
		ExposedPorts: nat.PortSet{
			nat.Port(strconv.Itoa(i.config.Port)): struct{}{},
		},
	}

	hconfig := &dcontainer.HostConfig{
		AutoRemove:      true,
		PublishAllPorts: true,
		RestartPolicy:   dcontainer.RestartPolicy{},
	}
	nconfig := &network.NetworkingConfig{}

	cinfo, err := i.node.Client().ContainerCreate(ctx, cconfig, hconfig, nconfig, "")
	if err != nil {
		i.l.WithError(err).Error("container-create")
		return "", err
	}

	return cinfo.ID, nil

}

func (i *instance) start(cid string) (dtypes.ContainerJSON, error) {
	// todo: timeout
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runch := runner.Do(func() runner.Result {
		return runner.NewResult(i.doStart(ctx, cid))
	})

	select {
	case err := <-i.lc.ShutdownRequest():
		res := <-runch
		if val, ok := res.Value().(dtypes.ContainerJSON); ok {
			return val, err
		}
		return dtypes.ContainerJSON{}, err
	case res := <-runch:
		if res.Err() != nil {
			return dtypes.ContainerJSON{}, res.Err()
		}
		return res.Value().(dtypes.ContainerJSON), nil
	}
}

func (i *instance) doStart(ctx context.Context, cid string) (dtypes.ContainerJSON, error) {

	if err := i.node.Client().ContainerStart(ctx, cid, dtypes.ContainerStartOptions{}); err != nil {
		i.l.WithError(err).Error("container-start")
		return dtypes.ContainerJSON{}, err
	}

	info, err := i.node.Client().ContainerInspect(ctx, cid)
	if err != nil {
		i.l.WithError(err).Error("container-inspect")
		return info, err
	}

	// emit started, state initialize
	return info, nil
}

func (i *instance) kill(cid string) error {
	// todo: timeout
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := i.node.Client().ContainerKill(ctx, cid, "KILL"); err != nil {
		i.l.WithError(err).Warn("kill")
		return err
	}
	return nil
}
