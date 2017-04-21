package ephemerald

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/ui"
	"github.com/docker/docker/api/types"
)

var (
	errImagePull      = fmt.Errorf("error pulling docker image")
	errNotRunning     = fmt.Errorf("pool not running")
	errNotInitialized = fmt.Errorf("pool not initialized")
)

type Pool interface {
	Checkout() (params.Params, error)
	CheckoutWith(context.Context) (params.Params, error)
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

type poolEventID string

const (
	eventItemReady    poolEventID = "ready"
	eventItemReturned poolEventID = "returned"
	eventItemExit     poolEventID = "exit"
)

type poolState string

const (
	stateInitializing poolState = "initializing"
	stateRunning      poolState = "running"
	stateShutdown     poolState = "shutdown"
)

type poolEvent struct {
	id   poolEventID
	item Item
}

type pool struct {
	state poolState

	config *config.Config

	// number of items to maintain
	size int

	// docker client
	adapter dockerAdapter

	// mainloop events
	events chan poolEvent

	// manages creating new items
	spawner poolItemSpawner

	// buffer of ready items
	readybuf poolItemBuffer

	// closed when initialization complete
	initch chan bool

	// closed when shutdown initiated.
	shutdownch chan bool

	// error during initialization
	initErr error

	// closed when completely shut down
	donech chan bool

	// "alive" items
	items map[string]poolItem

	ctx context.Context

	log logrus.FieldLogger

	emitter ui.PoolEmitter

	stopped int32
}

func NewPool(config *config.Config) (Pool, error) {
	return NewPoolWithContext(context.Background(), config)
}

func NewPoolWithContext(ctx context.Context, config *config.Config) (Pool, error) {

	adapter, err := newDockerAdapter(config)
	if err != nil {
		adapter.logger().WithError(err).Error("pool creation failed")
		return nil, err
	}

	log := adapter.logger().WithField("component", "Pool")

	emitter := config.Emitter()

	p := &pool{
		state:   stateInitializing,
		config:  config,
		size:    config.Size,
		adapter: adapter,

		readybuf: newPoolItemBuffer(emitter),
		spawner:  newPoolItemSpawner(emitter, adapter, config.Lifecycle),

		events: make(chan poolEvent),

		initch:     make(chan bool),
		shutdownch: make(chan bool),
		donech:     make(chan bool),

		items: make(map[string]poolItem),

		ctx: ctx,

		log:     log,
		emitter: emitter,
	}

	p.emitter.EmitInitializing()

	go p.run()
	go p.monitorCtx()

	return p, nil
}

func (p *pool) Stop() error {
	if !atomic.CompareAndSwapInt32(&p.stopped, 0, 1) {
		p.log.Warning("double stop")
		<-p.donech
		return nil
	}

	close(p.shutdownch)
	<-p.donech
	return nil
}

func (p *pool) WaitReady() error {
	select {
	case <-p.ctx.Done():
		return p.ctx.Err()
	case <-p.initch:
		return p.initErr
	}
}

func (p *pool) Checkout() (params.Params, error) {
	ctx, cancel := context.WithTimeout(p.ctx, p.defaultCheckoutTimeout())
	defer cancel()
	return p.CheckoutWith(ctx)
}

func (p *pool) CheckoutWith(ctx context.Context) (params.Params, error) {
	select {
	case <-ctx.Done():
		return params.Params{}, ctx.Err()
	case item, ok := <-p.readybuf.get():
		if !ok {
			return params.Params{}, errNotRunning
		}
		result, err := p.adapter.makeParams(item)
		if err != nil {
			p.Return(item)
			return params.Params{}, err
		}
		lcid(p.log, item.ID()).Info("checked out")
		return result, nil
	}
}

func (p *pool) Return(i Item) {
	p.events <- poolEvent{eventItemReturned, i}
}

func (p *pool) defaultCheckoutTimeout() time.Duration {
	return p.config.Lifecycle.MaxDelay() + (500 * time.Millisecond)
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
		p.emitter.EmitInitializeError(err)
		p.Stop()
	} else {
		p.emitter.EmitRunning()
	}

	p.runRunning()

	p.emitter.EmitDraining()

	p.runDrainSpawner()
	p.runDrainItems()

	p.emitter.EmitDone()
}

func (p *pool) runInitialize() error {
	defer close(p.initch)

	err := p.adapter.ensureImage()

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

			p.readybuf.stop()
			p.spawner.stop()

			p.killItems()

			return

		case item := <-p.spawner.next():
			p.items[item.ID()] = item
			item.join(p.events)
			item.start()
			p.emitter.EmitNumItems(len(p.items))

		case e := <-p.events:

			p.debugEvent(e, "running")

			// running; maintain item lifecycle
			switch e.id {

			case eventItemReady:
				if i, ok := p.items[e.item.ID()]; ok {
					p.readybuf.put(i)
				}

			case eventItemReturned:
				if i, ok := p.items[e.item.ID()]; ok {
					lcid(p.log, e.item.ID()).Info("returned")
					i.reset()
				}

			case eventItemExit:
				delete(p.items, e.item.ID())
				p.emitter.EmitNumItems(len(p.items))
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
		case item, ok := <-p.spawner.next():
			if !ok {
				p.log.Debugf("spawner drained.")
				return
			}
			item.kill()
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
			i.kill()
		}
	case eventItemExit:
		delete(p.items, e.item.ID())
		p.emitter.EmitNumItems(len(p.items))
	}
}

func (p *pool) primeBacklog() {
	if current := len(p.items); current < p.size {
		p.spawner.request(p.size - current)
	}
}

func (p *pool) killItems() {
	for _, c := range p.items {
		c.kill()
	}
}

func (p *pool) debugEvent(e poolEvent, msg string) {
	if e.item != nil {
		lcid(p.log, e.item.ID()).Debugf("%v: received event: %v", msg, e.id)
	} else {
		p.log.Debugf("%v: received event: %v", msg, e.id)
	}
}
