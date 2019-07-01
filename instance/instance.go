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
	Checkout(context.Context) (*types.Checkout, error)
	Release(context.Context) error
	Shutdown()
	Done() <-chan struct{}
}

type Config struct {
	PoolID    types.ID
	Image     reference.Canonical
	Port      int
	Container config.Container
	Params    map[string]string
	Actions   lifecycle.Config
	MaxResets int // TODO
}

func Create(ctx context.Context, bus pubsub.Bus, node node.Node, config Config) (Instance, error) {

	id, err := types.NewID()
	if err != nil {
		return nil, err
	}

	l := log.FromContext(ctx).
		WithField("cmp", "instance").
		WithField("pid", config.PoolID).
		WithField("iid", id)

	i := &instance{
		id: id,
		model: &types.Instance{
			ID:        id,
			PoolID:    config.PoolID,
			State:     types.InstanceStateCreate,
			MaxResets: config.MaxResets,
			Host:      node.Endpoint(),
		},
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
	model *types.Instance

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
	pch chan<- *types.Checkout
	ech chan<- error
	ctx context.Context
}

func (i *instance) Checkout(ctx context.Context) (*types.Checkout, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pch := make(chan *types.Checkout)
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
	case val := <-pch:
		return val, nil
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
		cid      string
		cinfo    dtypes.ContainerJSON
		actionch <-chan error
		ok       bool
	)

	if err := i.publishAction(types.EventActionStart); err != nil {
		i.lc.ShutdownInitiated(err)
		return
	}

	defer i.publishResult()

	actions, err := lifecycle.CreateActions(*i.model, i.bus, &i.config.Actions)
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

	cid, ok, err = i.create()
	if !ok {
		i.lc.ShutdownInitiated(err)
		goto kill
	}

	cinfo, ok, err = i.start(cid)
	if !ok {
		i.lc.ShutdownInitiated(err)
		goto kill
	}

	i.model.Port = tcpPortFor(cinfo, i.config.Port)

	actionch = i.runAction(types.InstanceStateCheck, ctx, actions.DoLive)

loop:
	for {
		select {
		case err := <-i.lc.ShutdownRequest():
			i.lc.ShutdownInitiated(err)
			break loop

		case req := <-i.checkoutch:

			if i.model.State != types.InstanceStateReady {
				i.l.Warn("checkout: invalid state")
				req.ech <- errors.New("invalid state")
				continue loop
			}

			co, err := i.newParams().ToCheckout()
			if err != nil {
				req.ech <- err
				i.l.WithError(err).Error("checkout: params->checkout failure")
				i.lc.ShutdownInitiated(err)
				break loop
			}

			select {
			case <-req.ctx.Done():
				i.l.Warn("checkout: stale request")
				continue loop
			case req.pch <- co:
			}

			i.enterState(types.InstanceStateCheckout)

		case req := <-i.releasech:

			if i.model.State != types.InstanceStateCheckout {
				i.l.Warn("release: invalid state")
				req <- errors.New("invalid state")
				continue loop
			}
			req <- nil

			if !actions.HasReset() {
				i.lc.ShutdownInitiated(err)
				break loop
			}

			actionch = i.runAction(types.InstanceStateReset, ctx, actions.DoReset)

		case err := <-actionch:
			actionch = nil
			switch i.model.State {
			case types.InstanceStateCheck:
				if err != nil {
					i.lc.ShutdownInitiated(err)
					break loop
				}

				actionch = i.runAction(types.InstanceStateInitialize, ctx, actions.DoInit)

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
				i.model.Resets++
				actionch = i.runAction(types.InstanceStateInitialize, ctx, actions.DoInit)
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

func (i *instance) runAction(state types.InstanceState, ctx context.Context, fn func(context.Context, params.Params) error) <-chan error {
	i.enterState(state)
	p := i.newParams()
	errch := make(chan error, 1)
	go func() {
		errch <- fn(ctx, p)
	}()
	return errch
}

func (i *instance) newParams() params.Params {
	return params.Create(*i.model, i.config.Params)
}

func (i *instance) publishAction(action types.EventAction) error {
	err := i.bus.Publish(types.Event{
		Type:     types.EventTypeInstance,
		Action:   action,
		Instance: &(*i.model),
		Status:   types.StatusInProgress,
	})
	if err != nil {
		i.l.WithError(err).
			WithField("action", action).
			Error("publish action")
	}
	return err
}

func (i *instance) publishResult() {

	ev := types.Event{
		Type:     types.EventTypeInstance,
		Action:   types.EventActionDone,
		Instance: &(*i.model),
	}

	if err := i.lc.Error(); err != nil {
		ev.Status = types.StatusFailure
		ev.Message = err.Error()
	} else {
		ev.Status = types.StatusSuccess
	}

	if err := i.bus.Publish(ev); err != nil {
		i.l.WithError(err).
			WithField("status", ev.Status).
			WithField("message", ev.Message).
			Error("publish result")
	}
}

func (i *instance) enterState(state types.InstanceState) {
	var err error

	i.model.State = state

	if state == types.InstanceStateReady {
		err = i.publishAction(types.EventActionReady)
	} else {
		err = i.publishAction(types.EventActionEnterState)
	}

	if err != nil {
		i.l.WithError(err).
			WithField("state", state).
			Error("publish enter state")
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

func (i *instance) create() (string, bool, error) {
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
		return res.Value().(string), false, err
	case res := <-runch:
		return res.Value().(string), res.Err() == nil, res.Err()
	}
}

func (i *instance) doCreate(ctx context.Context) (string, error) {
	cconfig := &dcontainer.Config{
		Labels: map[string]string{
			node.LabelEphemeraldPoolID:      string(i.PoolID()),
			node.LabelEphemeraldContainerID: string(i.id),
		},
		Image:        i.config.Image.Digest().String(),
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

func (i *instance) start(cid string) (dtypes.ContainerJSON, bool, error) {
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
			return val, false, err
		}
		return dtypes.ContainerJSON{}, false, err
	case res := <-runch:
		if res.Err() != nil {
			return dtypes.ContainerJSON{}, false, res.Err()
		}
		return res.Value().(dtypes.ContainerJSON), true, nil
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
