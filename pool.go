package cpool

import (
	"context"
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/registry"
)

const (
	checkoutTimeout = LiveCheckDefaultDelay * LiveCheckDefaultRetries
)

type Pool interface {
	Checkout() (StatusItem, error)
	CheckoutWith(context.Context) (StatusItem, error)
	Return(Item)
	Stop() error
}

type Item interface {
	ID() string
}

type StatusItem interface {
	Item
	Status() types.ContainerJSON
}

type eventId string

const (
	eventCreated       eventId = "created"
	eventStarted       eventId = "started"
	eventStartFailed   eventId = "start-failed"
	eventLive          eventId = "live"
	eventLiveErr       eventId = "live-err"
	eventInitializeErr eventId = "initialize-err"
	eventResetErr      eventId = "reset-err"
	eventReady         eventId = "ready"
	eventExitSuccess   eventId = "exit-success"
	eventExitError     eventId = "exit-error"
	eventReturned      eventId = "returned"
	eventCheckedOut    eventId = "checked-out"
	eventShutDown      eventId = "shut-down"

	eventImagePulled  eventId = "image-pulled"
	eventImagePullErr eventId = "image-pull-error"
)

type poolState string

const (
	stateInitializing poolState = "initializing"
	stateImagePullErr poolState = "image-pull-error"
	stateRunning      poolState = "running"
	stateShuttingDown poolState = "shutting-down"
	stateShutdown     poolState = "shutdown"
)

type event struct {
	id    eventId
	child *child
}

type pool struct {
	state poolState

	config      *Config
	size        int
	provisioner Provisioner

	client *client.Client
	ref    reference.Named
	info   *registry.RepositoryInfo

	events chan event

	cond *sync.Cond

	children map[string]*child
	ready    map[string]*child
	taken    map[string]*child

	ctx    context.Context
	cancel context.CancelFunc

	log logrus.FieldLogger
}

func NewPool(config *Config, size int, provisioner Provisioner) (Pool, error) {
	return NewPoolWithContext(context.Background(), config, size, provisioner)
}

func NewPoolWithContext(ctx context.Context, config *Config, size int, provisioner Provisioner) (Pool, error) {

	log := logrus.StandardLogger().WithField("component", "pool")

	ref, err := reference.ParseNormalizedNamed(config.Image)
	if err != nil {
		log.WithField("fn", "NewPool").
			WithError(err).
			Errorf("Unable to parse image '%s'", config.Image)
		return nil, err
	}

	log = log.WithField("image", ref.String())

	info, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return nil, err
	}

	client, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	events := make(chan event)

	p := &pool{
		state: stateInitializing,

		config:      config,
		size:        size,
		provisioner: provisioner,

		client: client,
		ref:    ref,
		info:   info,

		events: events,

		cond: sync.NewCond(&sync.Mutex{}),

		children: make(map[string]*child),
		ready:    make(map[string]*child),
		taken:    make(map[string]*child),

		ctx:    ctx,
		cancel: cancel,

		log: log,
	}

	go p.run()

	return p, nil
}

func (p *pool) Stop() error {

	p.events <- event{eventShutDown, nil}

	ch := make(chan interface{})
	go func() {
		p.cond.L.Lock()
		defer p.cond.L.Unlock()
		defer close(ch)
		for p.ctx.Err() == nil && len(p.children) > 0 {
			p.cond.Wait()
		}
	}()

	select {
	case <-p.ctx.Done():
		return p.ctx.Err()
	case <-ch:
		p.cancel()
	}

	return nil
}

func (p *pool) WaitReady() error {
}

func (p *pool) Checkout() (StatusItem, error) {
	ctx, cancel := context.WithTimeout(p.ctx, checkoutTimeout)
	defer cancel()
	return p.CheckoutWith(ctx)
}

func (p *pool) CheckoutWith(ctx context.Context) (StatusItem, error) {

	ch := make(chan *child)

	go func() {
		defer close(ch)
		p.cond.L.Lock()
		defer p.cond.L.Unlock()

		for len(p.ready) == 0 && ctx.Err() == nil {
			p.cond.Wait()
		}

		if ctx.Err() != nil {
			return
		}

		// get a single child
		var next *child
		for _, next = range p.ready {
			break
		}
		delete(p.ready, next.id)

		p.events <- event{eventCheckedOut, next}

		select {
		case <-ctx.Done():
		case ch <- next:
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case next := <-ch:
		return next, nil
	}
	return nil, context.DeadlineExceeded
}

func (p *pool) Return(i Item) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	if rc, ok := p.taken[i.ID()]; ok {
		delete(p.taken, i.ID())
		p.events <- event{eventReturned, rc}
	}
}

func (p *pool) run() {
	p.runInitialize()

	if p.state != stateRunning {
		return
	}

	p.runRunning()
}

func (p *pool) runInitialize() error {

	for {
		if p.state == stateInitializing {
			go p.doPullImage()
		}

		select {
		case <-p.ctx.Done():
			p.state = stateShutdown
			close(p.events)
			return p.ctx.Err()

		case e := <-p.events:
			switch e.id {
			case eventImagePulled:
				p.state = stateRunning
				return nil
			case eventImagePullErr:
				p.state = stateImagePullErr
				p.cancel()
				close(p.events)
				return fmt.Errorf("error pulling image")
			}
		}
	}
}

func (p *pool) runRunning() {
	for {

		if p.state == stateRunning {
			go p.primeBacklog()
		}

		select {

		case <-p.ctx.Done():
			p.log.Debugf("shut down due to context timeout")
			p.state = stateShutdown
			close(p.events)
			return

		case e := <-p.events:

			if e.child != nil {
				lcid(p.log, e.child.id).Debugf("received event: %v", e.id)
			} else {
				p.log.Debugf("received event: %v", e.id)
			}

			if p.state == stateRunning {

				// running; maintain child lifecycle

				switch e.id {
				case eventShutDown:
					p.state = stateShuttingDown
					go p.stopChildren()

				case eventCreated:
					go e.child.doStart()

				case eventStarted:
					go p.onChildStarted(e.child)

				case eventLive:
					go p.onChildLive(e.child)

				case eventReady:
					func() {
						p.cond.L.Lock()
						defer p.cond.L.Unlock()
						p.ready[e.child.id] = e.child
						p.cond.Broadcast()
					}()
				case eventReturned:
					go p.onChildReturned(e.child)
				}
			} else {

				// not running; kill new children.

				switch e.id {
				case eventCreated:
					fallthrough
				case eventStarted:
					fallthrough
				case eventLive:
					fallthrough
				case eventReady:
					go e.child.kill()
				}
			}

			// applicable to all states:

			switch e.id {
			case eventInitializeErr:
				fallthrough
			case eventResetErr:
				go e.child.kill()

			case eventStartFailed:
				fallthrough
			case eventExitError:
				fallthrough
			case eventExitSuccess:
				p.purgeChild(e.child)
			}
		}
	}
}

func (p *pool) doPullImage() {
	if err := ensureImageExists(p, p.ref); err != nil {
		p.events <- event{eventImagePullErr, nil}
	}
	p.events <- event{eventImagePulled, nil}
}

func (p *pool) onChildStarted(c *child) {

	if prov, ok := isLiveCheckProvisioner(p.provisioner); ok {
		if err := prov.LiveCheck(p.ctx, c); err != nil {
			p.log.WithError(err).Error("error checking liveliness")
			p.events <- event{eventLiveErr, c}
			return
		}
		p.events <- event{eventLive, c}
		return
	}

	// no live check
	p.onChildLive(c)
}

func (p *pool) onChildLive(c *child) {
	if prov, ok := isInitializeProvisioner(p.provisioner); ok {
		if err := prov.Initialize(p.ctx, c); err != nil {
			p.log.WithError(err).Error("error initializing")
			p.events <- event{eventInitializeErr, c}
			return
		}
	}
	p.events <- event{eventReady, c}
}

func (p *pool) onChildReturned(c *child) {
	if prov, ok := isResetProvisioner(p.provisioner); ok {
		if err := prov.Reset(p.ctx, c); err != nil {
			p.log.WithError(err).Error("error provisioning")
			p.events <- event{eventResetErr, c}
		}
	}
	c.events <- event{eventReady, c}
}

func (p *pool) purgeChild(c *child) {
	c.cancel()
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	delete(p.children, c.id)
	p.cond.Broadcast()
}

func (p *pool) stopChildren() {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	for _, child := range p.children {
		child.kill()
	}
}

func (p *pool) primeBacklog() {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()

	current := len(p.children)

	for ; current < p.size && p.ctx.Err() == nil; current++ {
		child, err := createChildFor(p)
		if err != nil {
			p.log.WithError(err).Error("can't create child; abort filling backlog")
			break
		}

		p.children[child.id] = child

		go func() {
			p.events <- event{eventCreated, child}
		}()
	}
	p.cond.Broadcast()
}
