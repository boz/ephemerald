package ephemerald

import (
	"context"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald/lifecycle"
	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/ui"
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

type poolItem interface {
	StatusItem
	join(ch chan<- poolEvent)
	start()
	reset()
	kill()
}

type pitem struct {
	lifecycle lifecycle.ContainerManager
	adapter   dockerAdapter
	container poolContainer

	events chan poolItemEvent
	joinch chan (chan<- poolEvent)

	// closed when exited
	exited chan bool

	ctx    context.Context
	cancel context.CancelFunc

	wg  sync.WaitGroup
	log logrus.FieldLogger

	uie ui.ContainerEmitter
}

func createPoolItem(uie ui.PoolEmitter, log logrus.FieldLogger, adapter dockerAdapter, lifecycle lifecycle.Manager) (poolItem, error) {
	log = log.WithField("component", "pool-item")

	container, err := createPoolContainer(log, adapter)
	if err != nil {
		log.WithError(err).
			Error("can't create container")
		return nil, err
	}

	log = lcid(log, container.ID())

	ctx, cancel := context.WithCancel(context.Background())

	cuie := uie.ForContainer(container.ID())

	cuie.EmitCreated()

	item := &pitem{
		lifecycle: lifecycle.ForContainer(cuie, container.ID()),
		adapter:   adapter,
		container: container,
		events:    make(chan poolItemEvent),
		joinch:    make(chan (chan<- poolEvent)),
		exited:    make(chan bool),
		ctx:       ctx,
		cancel:    cancel,
		log:       log,
		uie:       cuie,
	}

	go item.run()

	return item, nil
}

func (i *pitem) ID() string {
	return i.container.ID()
}

func (i *pitem) Status() types.ContainerJSON {
	return i.container.Status()
}

func (i *pitem) join(ch chan<- poolEvent) {
	i.joinch <- ch
}

func (i *pitem) start() {
	go i.sendEvent(eventPoolItemStart)
}

func (i *pitem) reset() {
	go i.sendEvent(eventPoolItemReset)
}

func (i *pitem) kill() {
	go i.sendEvent(eventPoolItemKill)
}

func (i *pitem) sendEvent(e poolItemEvent) {
	select {
	case <-i.exited:
	case i.events <- e:
	}
}

func (i *pitem) run() {
	ch := i.runWaitJoin()
	i.runMainLoop(ch)

	i.drain()
}

func (i *pitem) runWaitJoin() chan<- poolEvent {
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
				i.uie.EmitExiting()
				i.container.stop()
			}
		}
	}
}

func (i *pitem) runMainLoop(ch chan<- poolEvent) {
	defer close(i.exited)
	defer i.cancel()

	log := i.log.WithField("method", "runMainLoop")

	for {
		select {
		case e := <-i.container.events():
			log.WithField("event", e).Debug("container-event")

			switch e {
			case containerEventExitSuccess:
				fallthrough
			case containerEventExitError:
				fallthrough
			case containerEventStartFailed:
				i.log.Info("container exited")
				i.uie.EmitExited()
				ch <- poolEvent{eventItemExit, i}
				return
			case containerEventStarted:
				i.uie.EmitStarted()
				i.do(i.onChildStarted)
			}

		case e := <-i.events:
			log.WithField("event", e).Debug("item-event")

			switch e {
			case eventPoolItemKill:
				i.uie.EmitExiting()
				i.container.stop()
			case eventPoolItemStart:
				i.container.start()
			case eventPoolItemLive:
				i.uie.EmitLive()
				i.do(i.onChildLive)
			case eventPoolItemLiveError:
				i.uie.EmitExiting()
				i.container.stop()
			case eventPoolItemReady:
				i.uie.EmitReady()
				ch <- poolEvent{eventItemReady, i}
			case eventPoolItemReadyError:
				i.uie.EmitExiting()
				i.container.stop()
			case eventPoolItemReset:
				i.uie.EmitResetting()
				i.do(i.onChildReset)
			}

		}
	}
}

func (i *pitem) drain() {
	log := i.log.WithField("method", "drain")

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

func (i *pitem) onChildStarted() {
	if i.lifecycle.HasHealthcheck() {
		params, err := i.currentParams()
		if err != nil {
			i.events <- eventPoolItemLiveError
			return
		}
		if err := i.lifecycle.DoHealthcheck(i.ctx, params); err != nil {
			i.log.WithError(err).Error("error checking liveliness")
			i.events <- eventPoolItemLiveError
			return
		}
	}
	i.events <- eventPoolItemLive
}

func (i *pitem) onChildLive() {
	if i.lifecycle.HasInitialize() {
		params, err := i.currentParams()
		if err != nil {
			i.events <- eventPoolItemReadyError
			return
		}
		if err := i.lifecycle.DoInitialize(i.ctx, params); err != nil {
			i.log.WithError(err).Error("error initializing")
			i.events <- eventPoolItemReadyError
			return
		}
	}
	i.log.Info("container ready")
	i.events <- eventPoolItemReady
}

func (i *pitem) onChildReset() {
	if i.lifecycle.HasReset() {
		params, err := i.currentParams()
		if err != nil {
			i.events <- eventPoolItemResetError
			return
		}
		if err := i.lifecycle.DoReset(i.ctx, params); err != nil {
			i.log.WithError(err).Error("error provisioning")
			i.events <- eventPoolItemResetError
			return
		}
		i.events <- eventPoolItemReady
		return
	}
	i.container.stop()
}

func (i *pitem) currentParams() (params.Params, error) {
	params, err := i.adapter.makeParams(i.container)
	if err != nil {
		i.log.WithError(err).Warn("error making params")
	}
	return params, err
}

func (i *pitem) do(fn func()) {
	i.wg.Add(1)
	go func() {
		defer i.wg.Done()
		fn()
	}()
}
