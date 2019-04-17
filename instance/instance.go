package instance

import (
	"context"
	"errors"
	"strconv"

	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/lifecycle"
	"github.com/boz/ephemerald/log"
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
	"github.com/sirupsen/logrus"
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
	Params    params.Config
	Actions   lifecycle.Config
	MaxResets int // TODO
}

func Create(bus pubsub.Bus, node node.Node, config Config) (Instance, error) {

	id, err := types.NewID()
	if err != nil {
		return nil, err
	}

	l := log.New().
		WithField("cmp", "instance").
		WithField("pid", config.PoolID).
		WithField("iid", id)

	i := &instance{
		id:         id,
		state:      types.InstanceStateStart,
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
	state types.InstanceState

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
		return nil, ctx.Err()
	case <-i.lc.ShuttingDown():
		return nil, errors.New("not running")
	case i.checkoutch <- req:
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-i.lc.ShuttingDown():
		return nil, errors.New("not running")
	case err := <-ech:
		return nil, err
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
		iparams  params.Params
		cid      string
		cinfo    dtypes.ContainerJSON
		actionch <-chan error
	)

	actions, err := lifecycle.CreateActions(&i.config.Actions)
	if err != nil {
		i.lc.ShutdownInitiated(err)
		return
	}

	sub, err := i.subscribe()
	if err != nil {
		i.lc.ShutdownInitiated(err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

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

	iparams = params.Create(params.State{
		ID:   i.id,
		Host: i.node.Endpoint(),
		Port: tcpPortFor(cinfo, i.config.Port),
	}, i.config.Params)

	i.enterState(types.InstanceStateCheck)
	actionch = i.runAction(ctx, iparams, actions.DoReady)

loop:
	for {
		select {
		case err := <-i.lc.ShutdownRequest():
			i.lc.ShutdownInitiated(err)
			break loop

		case req := <-i.checkoutch:

			if i.state != types.InstanceStateReady {
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

			i.enterState(types.InstanceStateCheckout)

		case req := <-i.releasech:

			if i.state != types.InstanceStateCheckout {
				i.l.Warn("release: invalid state")
				req <- errors.New("invalid state")
				continue loop
			}
			req <- nil

			if !actions.HasReset() {
				i.lc.ShutdownInitiated(err)
				break loop
			}

			i.enterState(types.InstanceStateReset)
			actionch = i.runAction(ctx, iparams, actions.DoReset)

		case err := <-actionch:
			actionch = nil
			switch i.state {
			case types.InstanceStateCheck:
				if err != nil {
					i.lc.ShutdownInitiated(err)
					break loop
				}

				i.enterState(types.InstanceStateInitialize)
				actionch = i.runAction(ctx, iparams, actions.DoInit)

			case types.InstanceStateInitialize:
				if err != nil {
					i.lc.ShutdownInitiated(err)
					break loop
				}

				i.enterState(types.InstanceStateReady)

			case types.InstanceStateReset:
				if err != nil {
					i.lc.ShutdownInitiated(err)
					break loop
				}

				i.enterState(types.InstanceStateInitialize)
				actionch = i.runAction(ctx, iparams, actions.DoInit)
			}
		}
	}

kill:
	cancel()

	sub.Close()

	i.kill(cid)

	if actionch != nil {
		<-actionch
	}

	i.enterState(types.InstanceStateDone)

	<-sub.Done()
}

func (i *instance) enterState(state types.InstanceState) {
	i.state = state
	err := i.bus.Publish(types.Event{
		Type:     types.EventTypeInstance,
		Action:   types.EventActionEnterState,
		Pool:     i.PoolID(),
		Instance: i.id,
	})
	if err != nil {
		i.l.WithError(err).
			WithField("state", state).
			Error("entering state")
	}
}

func (i *instance) subscribe() (pubsub.Subscription, error) {
	filter := func(ev types.BusEvent) bool {
		return ev.GetInstanceID() == i.id &&
			ev.GetPoolID() == i.PoolID() &&
			ev.GetType() != types.EventTypeInstance
	}
	sub, err := i.bus.Subscribe(filter)
	if err != nil {
		i.l.WithError(err).Error("bus-subscribe")
	}
	return sub, err
}

func (i *instance) create() (string, error) {
	i.enterState(types.InstanceStateCreate)

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
			nat.Port(strconv.Itoa(i.config.Port) + "/tcp"): struct{}{},
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
	i.enterState(types.InstanceStateStart)

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

	return info, nil
}

func (i *instance) kill(cid string) error {
	i.enterState(types.InstanceStateKill)

	// todo: timeout
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := i.node.Client().ContainerKill(ctx, cid, "KILL"); err != nil {
		i.l.WithError(err).Warn("kill")
		return err
	}
	return nil
}

func (i *instance) runAction(ctx context.Context, iparams params.Params, fn func(context.Context, params.Params) error) <-chan error {
	errch := make(chan error, 1)
	go func() {
		errch <- fn(ctx, iparams)
	}()
	return errch
}
