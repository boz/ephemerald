package ephemerald

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
)

const (
	checkoutTimeout = LiveCheckDefaultDelay * LiveCheckDefaultRetries
)

var (
	errImagePull      = fmt.Errorf("error pulling docker image")
	errNotRunning     = fmt.Errorf("pool not running")
	errNotInitialized = fmt.Errorf("pool not initialized")
)

type Pool interface {
	Checkout() (StatusItem, error)
	CheckoutWith(context.Context) (StatusItem, error)
	Return(Item)
	Stop() error
	WaitReady() error
}

type Item interface {
	ID() string
}

type StatusItem interface {
	Item
	Status() types.ContainerJSON
}

type PoolItem interface {
	StatusItem
	Join(ch chan<- poolEvent)
	Start()
	Reset()
	Kill()
}

type poolEventId string

const (
	eventItemReady    poolEventId = "ready"
	eventItemReturned poolEventId = "returned"
	eventItemExit     poolEventId = "exit"
)

type poolState string

const (
	stateInitializing poolState = "initializing"
	stateRunning      poolState = "running"
	stateShutdown     poolState = "shutdown"
)

type poolEvent struct {
	id   poolEventId
	item Item
}

type pool struct {
	state poolState

	config      *Config
	provisioner Provisioner

	// number of items to maintain
	size int

	// docker client
	adapter Adapter

	// mainloop events
	events chan poolEvent

	// manages creating new items
	spawner *spawner

	// buffer of ready items
	readybuf *itembuf

	// closed when initialization complete
	initch chan bool

	// closed when shutdown initiated.
	shutdownch chan bool

	// error during initialization
	initErr error

	// closed when completely shut down
	donech chan bool

	// "alive" items
	items map[string]PoolItem

	ctx context.Context

	log logrus.FieldLogger

	stopped int32
}

func NewPool(
	config *Config, size int, provisioner Provisioner) (Pool, error) {
	return NewPoolWithContext(context.Background(), config, size, provisioner)
}

func NewPoolWithContext(
	ctx context.Context, config *Config, size int, provisioner Provisioner) (Pool, error) {

	log := logrus.New().WithField("component", "pool")
	log.Level = config.LogLevel

	adapter, err := newAdapter(log, config)
	if err != nil {
		log.WithError(err).Error("pool creation failed")
		return nil, err
	}

	log = log.WithField("image", adapter.Ref().String())

	p := &pool{
		state:       stateInitializing,
		config:      config,
		provisioner: provisioner,
		size:        size,
		adapter:     adapter,

		readybuf: newItemBuffer(),
		spawner:  newSpawner(log, adapter, provisioner),

		events: make(chan poolEvent),

		initch:     make(chan bool),
		shutdownch: make(chan bool),
		donech:     make(chan bool),

		items: make(map[string]PoolItem),

		ctx: ctx,

		log: log,
	}

	go p.run()
	go p.monitorCtx()

	return p, nil
}

func (p *pool) Stop() error {

	if !atomic.CompareAndSwapInt32(&p.stopped, 0, 1) {
		p.log.Warning("double stop")
		return errNotRunning
	}

	close(p.shutdownch)

	select {
	case <-p.ctx.Done():
		return p.ctx.Err()
	case <-p.donech:
		return nil
	}
}

func (p *pool) WaitReady() error {
	select {
	case <-p.ctx.Done():
		return p.ctx.Err()
	case <-p.initch:
		return p.initErr
	}
}

func (p *pool) Checkout() (StatusItem, error) {
	ctx, cancel := context.WithTimeout(p.ctx, checkoutTimeout)
	defer cancel()
	return p.CheckoutWith(ctx)
}

func (p *pool) CheckoutWith(ctx context.Context) (StatusItem, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case item, ok := <-p.readybuf.Get():
		if !ok {
			return nil, errNotRunning
		}
		return item, nil
	}
}

func (p *pool) Return(i Item) {
	p.events <- poolEvent{eventItemReturned, i}
}

func (p *pool) monitorCtx() {
	select {
	case <-p.ctx.Done():
		p.Stop()
	case <-p.shutdownch:
	}
}

func (p *pool) run() {
	defer close(p.donech)

	err := p.runInitialize()

	if err != nil {
		p.Stop()
	}

	p.runRunning()
	p.runDrainSpawner()
	p.runDrainItems()
}

func (p *pool) runInitialize() error {
	defer close(p.initch)

	err := p.adapter.EnsureImage()

	if err != nil {
		p.state = stateShutdown
		p.initErr = err
		return err
	}

	p.state = stateRunning

	p.primeBacklog()

	return nil
}

func (p *pool) runRunning() {
	for {
		select {
		case <-p.shutdownch:

			p.state = stateShutdown

			p.readybuf.Stop()
			p.spawner.Stop()

			p.killItems()

			return

		case item := <-p.spawner.Next():
			p.items[item.ID()] = item
			item.Join(p.events)
			item.Start()

		case e := <-p.events:

			p.debugEvent(e, "running")

			// running; maintain item lifecycle
			switch e.id {

			case eventItemReady:
				if i, ok := p.items[e.item.ID()]; ok {
					p.readybuf.Put(i)
				}

			case eventItemReturned:
				if i, ok := p.items[e.item.ID()]; ok {
					i.Reset()
				}

			case eventItemExit:
				delete(p.items, e.item.ID())
				p.primeBacklog()
			}
		}
	}
}

func (p *pool) runDrainSpawner() {
	p.log.Debugf("draining spawner")

	for {
		p.killItems()
		select {
		case item, ok := <-p.spawner.Next():
			if !ok {
				p.log.Debugf("spawner drained.")
				return
			}
			item.Kill()
		case e := <-p.events:
			p.handleDrainingEvent(e, "drain-spawn")
		}
	}
}

func (p *pool) runDrainItems() {
	p.log.Debugf("draining items")
	for {

		if len(p.items) == 0 {
			return
		}

		p.killItems()

		select {
		case e := <-p.events:
			p.handleDrainingEvent(e, "drain-chilren")
		}
	}
}

func (p *pool) handleDrainingEvent(e poolEvent, msg string) {
	p.debugEvent(e, msg)
	switch e.id {
	case eventItemReady:
		fallthrough
	case eventItemReturned:
		if i, ok := p.items[e.item.ID()]; ok {
			i.Kill()
		}
	case eventItemExit:
		delete(p.items, e.item.ID())
	}
}

func (p *pool) primeBacklog() {
	if current := len(p.items); current < p.size {
		p.spawner.Request(p.size - current)
	}
}

func (p *pool) killItems() {
	for _, c := range p.items {
		c.Kill()
	}
}

func (p *pool) debugEvent(e poolEvent, msg string) {
	if e.item != nil {
		lcid(p.log, e.item.ID()).Debugf("%v: received event: %v", msg, e.id)
	} else {
		p.log.Debugf("%v: received event: %v", msg, e.id)
	}
}
