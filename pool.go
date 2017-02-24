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

	eventImagePulled  eventId = "image-pulled"
	eventImagePullErr eventId = "image-pull-error"
)

type poolState string

const (
	stateInitializing poolState = "initializing"
	stateRunning      poolState = "running"
	stateShutdown     poolState = "shutdown"
)

type event struct {
	id    eventId
	child *child
}

type pool struct {
	state poolState
	err   error

	config      *Config
	size        int
	provisioner Provisioner

	client *client.Client
	ref    reference.Named
	info   *registry.RepositoryInfo

	events  chan event
	statech chan poolState
	childch chan *child
	readych chan *child

	cond *sync.Cond

	children map[string]*child
	ready    map[string]*child
	taken    map[string]*child

	ctx    context.Context
	cancel context.CancelFunc

	log logrus.FieldLogger
}

func NewPool(
	config *Config, size int, provisioner Provisioner) (Pool, error) {
	return NewPoolWithContext(context.Background(), config, size, provisioner)
}

func NewPoolWithContext(
	ctx context.Context, config *Config, size int, provisioner Provisioner) (Pool, error) {

	log := logrus.StandardLogger().WithField("component", "pool")

	ref, err := reference.ParseNormalizedNamed(config.Image)
	if err != nil {
		log.WithError(err).
			Errorf("Unable to parse image '%s'", config.Image)
		return nil, err
	}

	log = log.WithField("image", ref.String())

	info, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		log.WithError(err).Error("unable to parse repository info")
		return nil, err
	}

	client, err := client.NewEnvClient()
	if err != nil {
		log.WithError(err).Error("unable to crate docker client")
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	p := &pool{
		state: stateInitializing,

		config:      config,
		size:        size,
		provisioner: provisioner,

		client: client,
		ref:    ref,
		info:   info,

		events:  make(chan event),
		statech: make(chan poolState, 1),
		childch: make(chan *child, 1),
		readych: make(chan *child, 1),

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
	defer close(p.events)
	defer close(p.statech)
	defer close(p.childch)
	defer close(p.readych)
	defer p.cancel()

	p.setTerminalState(stateShutdown, nil)

	check := func() (int, error) {
		p.cond.L.Lock()
		defer p.cond.L.Unlock()
		return len(p.children), p.err
	}

	for {
		count, err := check()
		if count == 0 {
			return err
		}

		select {
		case <-p.ctx.Done():
			return p.ctx.Err()
		case <-p.childch:
		}
	}
}

func (p *pool) WaitReady() error {

	check := func() (bool, error) {
		p.cond.L.Lock()
		defer p.cond.L.Unlock()
		state, err := p.state, p.err
		switch state {
		case stateInitializing:
			return false, err
		default:
			return true, err
		}
	}

	for {
		done, err := check()
		if done {
			return err
		}
		select {
		case <-p.ctx.Done():
			return p.ctx.Err()
		case <-p.statech:
		}
	}
}

func (p *pool) Checkout() (StatusItem, error) {
	ctx, cancel := context.WithTimeout(p.ctx, checkoutTimeout)
	defer cancel()
	return p.CheckoutWith(ctx)
}

func (p *pool) CheckoutWith(ctx context.Context) (StatusItem, error) {

	check := func() (*child, error) {
		p.cond.L.Lock()
		defer p.cond.L.Unlock()

		if p.state == stateInitializing {
			return nil, errNotInitialized
		}

		if p.state != stateRunning {
			return nil, errNotRunning
		}

		if len(p.ready) == 0 {
			return nil, nil
		}

		// get a single child
		var next *child
		for _, next = range p.ready {
			break
		}
		delete(p.ready, next.id)
		p.taken[next.id] = next

		return next, nil
	}

	for {
		next, err := check()
		if next != nil || err != nil {
			return next, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-p.ctx.Done():
			return nil, p.ctx.Err()
		case <-p.readych:
		}
	}
}

func (p *pool) Return(i Item) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	if rc, ok := p.taken[i.ID()]; ok {
		delete(p.taken, i.ID())
		p.sendEvent(eventReturned, rc)
	}
}

func (p *pool) run() {
	p.runInitialize()

	if p.getState() != stateRunning {
		return
	}

	p.runRunning()
}

func (p *pool) runInitialize() error {

	go p.doPullImage()

	select {
	case <-p.ctx.Done():
		p.setTerminalState(stateShutdown, p.ctx.Err())
		return p.ctx.Err()

	case e, ok := <-p.events:

		if !ok {
			p.log.Debug("events closed; shutting down")
			return nil
		}

		switch e.id {
		case eventImagePulled:
			p.setState(stateRunning)
			return nil
		case eventImagePullErr:
			p.setTerminalState(stateShutdown, errImagePull)
			return errImagePull
		}
	}
	return nil
}

func (p *pool) runRunning() {
	for {

		if p.getState() == stateRunning {
			go p.primeBacklog()
		}

		select {

		case <-p.ctx.Done():
			p.setTerminalState(stateShutdown, p.ctx.Err())
			return

		case e, ok := <-p.events:

			if !ok {
				p.log.Debugf("p.events closed; shutting down")
				return
			}

			if e.child != nil {
				lcid(p.log, e.child.id).Debugf("received event: %v", e.id)
			} else {
				p.log.Debugf("received event: %v", e.id)
			}

			switch p.getState() {
			case stateRunning:
				// running; maintain child lifecycle
				switch e.id {

				case eventCreated:
					go p.onChildCreated(e.child)

				case eventStarted:
					go p.onChildStarted(e.child)

				case eventLive:
					go p.onChildLive(e.child)

				case eventReady:
					p.onChildReady(e.child)

				case eventReturned:
					go p.onChildReturned(e.child)
				}
			default:
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
		p.sendEvent(eventImagePullErr, nil)
		return
	}
	p.sendEvent(eventImagePulled, nil)
}

func (p *pool) onChildCreated(c *child) {
	c.doStart()
}

func (p *pool) onChildStarted(c *child) {

	if prov, ok := isLiveCheckProvisioner(p.provisioner); ok {
		if err := prov.LiveCheck(p.ctx, c); err != nil {
			p.log.WithError(err).Error("error checking liveliness")
			p.sendEvent(eventLiveErr, c)
			return
		}
		p.sendEvent(eventLive, c)
		return
	}

	// no live check
	p.onChildLive(c)
}

func (p *pool) onChildLive(c *child) {
	if prov, ok := isInitializeProvisioner(p.provisioner); ok {
		if err := prov.Initialize(p.ctx, c); err != nil {
			p.log.WithError(err).Error("error initializing")
			p.sendEvent(eventInitializeErr, c)
			return
		}
	}
	p.sendEvent(eventReady, c)
}

func (p *pool) onChildReady(c *child) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	if c, ok := p.children[c.id]; ok && p.state == stateRunning {
		p.ready[c.id] = c
		select {
		case p.readych <- c:
		default:
		}
	}
}

func (p *pool) onChildReturned(c *child) {
	if prov, ok := isResetProvisioner(p.provisioner); ok {
		if err := prov.Reset(p.ctx, c); err != nil {
			p.log.WithError(err).Error("error provisioning")
			p.sendEvent(eventResetErr, c)
		}
	}
	p.sendEvent(eventReady, c)
}

func (p *pool) purgeChild(c *child) {
	c.cancel()
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	delete(p.children, c.id)

	select {
	case p.childch <- c:
	default:
	}
}

func (p *pool) primeBacklog() {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()

	if p.state != stateRunning {
		return
	}

	current := len(p.children)

	for ; current < p.size && p.ctx.Err() == nil; current++ {
		child, err := createChildFor(p)
		if err != nil {
			p.log.WithError(err).Error("can't create child; abort filling backlog")
			break
		}

		p.children[child.id] = child

		select {
		case p.childch <- child:
		default:
		}

		go p.sendEvent(eventCreated, child)
	}
}

func (p *pool) setState(state poolState) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	p.state = state
	select {
	case p.statech <- state:
	default:
	}
}

func (p *pool) getState() poolState {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	return p.state
}

func (p *pool) setTerminalState(state poolState, err error) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()

	if p.state == state {
		return
	}

	p.state = state

	if p.err != nil {
		p.err = err
	}

	for _, child := range p.children {
		child.kill()
	}
	select {
	case p.statech <- state:
	default:
	}
}

func (p *pool) sendEvent(id eventId, c *child) {
	select {
	case <-p.ctx.Done():
	case p.events <- event{id, c}:
	}
}
