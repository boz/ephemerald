package ephemerald

import (
	"context"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
)

type poolItemEvent string

const (
	eventPoolItemStart poolItemEvent = "start"
	eventPoolItemReset poolItemEvent = "reset"
	eventPoolItemKill  poolItemEvent = "kill"

	eventPoolItemLiveError  poolItemEvent = "live-error"
	eventPoolItemLive       poolItemEvent = "live"
	eventPoolItemResetError poolItemEvent = "reset-error"
	eventPoolItemReady      poolItemEvent = "ready"
	eventPoolItemReadyError poolItemEvent = "ready-error"
)

type poolItemState string

const (
	poolItemStateRunning poolItemState = "running"
	poolItemStateExited  poolItemState = "exited"
)

type poolItem struct {
	state       poolItemState
	provisioner Provisioner
	adapter     Adapter
	container   PoolContainer

	events chan poolItemEvent
	joinch chan (chan<- poolEvent)

	// closed when exited
	exited chan bool

	ctx context.Context

	wg  sync.WaitGroup
	log logrus.FieldLogger
}

func createPoolItem(log logrus.FieldLogger, adapter Adapter, provisioner Provisioner) (PoolItem, error) {
	log = log.WithField("component", "pool-item")

	container, err := createPoolContainer(log, adapter)
	if err != nil {
		log.WithError(err).
			Error("can't create container")
		return nil, err
	}

	log = lcid(log, container.ID())

	item := &poolItem{
		state:       poolItemStateRunning,
		provisioner: provisioner,
		adapter:     adapter,
		container:   container,
		events:      make(chan poolItemEvent),
		joinch:      make(chan (chan<- poolEvent)),
		exited:      make(chan bool),
		ctx:         context.Background(),
		log:         log,
	}

	go item.run()

	return item, nil
}

func (c *poolItem) ID() string {
	return c.container.ID()
}

func (c *poolItem) Status() types.ContainerJSON {
	return c.container.Status()
}

func (i *poolItem) Join(ch chan<- poolEvent) {
	i.joinch <- ch
}

func (i *poolItem) Start() {
	select {
	case <-i.exited:
	case i.events <- eventPoolItemStart:
	}
}

func (i *poolItem) Reset() {
	select {
	case <-i.exited:
	case i.events <- eventPoolItemReset:
	}
}

func (i *poolItem) Kill() {
	select {
	case <-i.exited:
	case i.events <- eventPoolItemKill:
	}
}

func (i *poolItem) run() {
	ch := i.runWaitJoin()
	i.runMainLoop(ch)

	i.drain()
}

func (i *poolItem) runWaitJoin() chan<- poolEvent {
	defer close(i.joinch)

	log := i.log.WithField("method", "runWaitJoin")

	for {
		select {
		case ch := <-i.joinch:
			log.Debug("joined")
			return ch
		case e := <-i.events:
			log.WithField("event", e).Debug("item-event")
			switch e {
			case eventPoolItemKill:
				i.container.Stop()
			}
		}
	}
}

func (i *poolItem) runMainLoop(ch chan<- poolEvent) {
	defer close(i.exited)

	log := i.log.WithField("method", "runMainLoop")

	for {
		select {
		case e := <-i.container.Events():
			log.WithField("event", e).Debug("container-event")

			switch e {
			case containerEventExitSuccess:
				fallthrough
			case containerEventExitError:
				fallthrough
			case containerEventStartFailed:
				ch <- poolEvent{eventItemExit, i}
				return
			case containerEventStarted:
				i.do(i.onChildStarted)
			}

		case e := <-i.events:
			log.WithField("event", e).Debug("item-event")

			switch e {
			case eventPoolItemKill:
				i.container.Stop()
			case eventPoolItemStart:
				i.container.Start()
			case eventPoolItemLive:
				i.do(i.onChildLive)
			case eventPoolItemLiveError:
				i.container.Stop()
			case eventPoolItemReady:
				ch <- poolEvent{eventItemReady, i}
			case eventPoolItemReadyError:
				i.container.Stop()
			case eventPoolItemReset:
				i.do(i.onChildReset)
			}

		}
	}
}

func (i *poolItem) drain() {
	log := i.log.WithField("method", "drain")

	defer close(i.events)

	ch := make(chan bool)
	go func() {
		i.wg.Wait()
		close(ch)
	}()

	for {
		select {
		case <-ch:
			log.Debug("done")
			return
		case e := <-i.events:
			log.WithField("event", e).Debug("stale item-event")
		}
	}
}

func (i *poolItem) onChildStarted() {
	if prov, ok := isLiveCheckProvisioner(i.provisioner); ok {
		if err := prov.LiveCheck(i.ctx, i.container); err != nil {
			i.log.WithError(err).Error("error checking liveliness")
			i.events <- eventPoolItemLiveError
			return
		}
	}
	i.events <- eventPoolItemLive
}

func (i *poolItem) onChildLive() {
	if prov, ok := isInitializeProvisioner(i.provisioner); ok {
		if err := prov.Initialize(i.ctx, i.container); err != nil {
			i.log.WithError(err).Error("error initializing")
			i.events <- eventPoolItemReadyError
			return
		}
	}
	i.events <- eventPoolItemReady
}

func (i *poolItem) onChildReset() {
	if prov, ok := isResetProvisioner(i.provisioner); ok {
		if err := prov.Reset(i.ctx, i.container); err != nil {
			i.log.WithError(err).Error("error provisioning")
			i.events <- eventPoolItemResetError
		}
		i.events <- eventPoolItemReady
		return
	}
	i.container.Stop()
}

func (i *poolItem) do(fn func()) {
	i.wg.Add(1)
	go func() {
		defer i.wg.Done()
		fn()
	}()
}
