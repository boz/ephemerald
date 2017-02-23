package cpool

import (
	"context"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/client"
	"github.com/docker/docker/registry"
)

var (
	log = logrus.StandardLogger().
		WithField("package", "github.com/ovrclk/cpool").
		WithField("module", "pool")
)

type Pool interface {
	Checkout() StatusItem
	Return(Item)
	Stop() error
}

type eventId int

const (
	eventCreated eventId = iota
	eventStarted
	eventStartFailed
	eventExitSuccess
	eventExitError
	eventInitializeErr
	eventResetErr
	eventReady

	eventReturned
	eventCheckedOut
	eventShutDown
)

type event struct {
	id    eventId
	child *child
}

type pool struct {
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

	shutdown bool
}

func NewPool(config *Config, size int, provisioner Provisioner) (Pool, error) {

	ref, err := reference.ParseNormalizedNamed(config.Image)
	if err != nil {
		log.Errorf("Unable to parse image '%s'", config.Image)
		return nil, err
	}

	info, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return nil, err
	}

	client, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	events := make(chan event)

	p := &pool{
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
	}

	go p.run()

	return p, nil
}

func (p *pool) Stop() error {
	defer p.cancel()

	p.events <- event{eventShutDown, nil}

	ch := make(chan interface{})
	go func() {
		p.cond.L.Lock()
		defer p.cond.L.Unlock()
		defer close(ch)
		for p.ctx.Err() == nil && len(p.children) == 0 {
			p.cond.Wait()
		}
	}()

	select {
	case <-p.ctx.Done():
		return p.ctx.Err()
	case <-ch:
	}

	return nil
}

func (p *pool) Checkout() StatusItem {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()

	for len(p.ready) == 0 {
		p.cond.Wait()
	}

	// get a single child
	var next *child
	for _, next = range p.ready {
		break
	}
	delete(p.ready, next.id)

	p.events <- event{eventCheckedOut, next}
	return next
}

func (p *pool) Return(i Item) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	if rc, ok := p.taken[i.ID()]; ok {
		delete(p.taken, i.ID())
		p.events <- event{eventReturned, rc}
	}
}

func (p *pool) Fetch() error {
	return ensureImageExists(p, p.ref)
}

func (p *pool) run() {
	for {

		if !p.shutdown {
			p.primeBacklog()
		}

		select {

		case <-p.ctx.Done():
			close(p.events)
			return

		case e := <-p.events:

			log.Debug("received event: %+v", e)

			switch e.id {

			case eventShutDown:
				p.shutdown = true
				go p.stopChildren()

			case eventCreated:

				func() {
					p.cond.L.Lock()
					defer p.cond.L.Unlock()
					p.children[e.child.id] = e.child
				}()
				go e.child.doStart()

			case eventStarted:
				go p.onChildStarted(e.child)

			case eventStartFailed:
				// todo: retry?
				p.purgeChild(e.child)

			case eventInitializeErr:
				e.child.kill()

			case eventReady:
				func() {
					p.cond.L.Lock()
					defer p.cond.L.Unlock()
					p.ready[e.child.id] = e.child
					p.cond.Broadcast()
				}()

			case eventExitError:
				// todo: retry?
				p.purgeChild(e.child)

			case eventExitSuccess:
				p.purgeChild(e.child)

			case eventCheckedOut:
			case eventReturned:
				go p.onChildReturned(e.child)

			case eventResetErr:
				go e.child.kill()
			}
		}
	}
}

func (p *pool) onChildStarted(c *child) {
	if prov, ok := isInitializeProvisioner(p.provisioner); ok {
		if err := prov.Initialize(p.ctx, c); err != nil {
			log.WithError(err).Error("error initializing")
			p.events <- event{eventInitializeErr, c}
			return
		}
	}
	p.events <- event{eventReady, c}
}

func (p *pool) onChildReturned(c *child) {
	if prov, ok := isResetProvisioner(p.provisioner); ok {
		if err := prov.Reset(p.ctx, c); err != nil {
			log.WithError(err).Error("error provisioning")
			p.events <- event{eventResetErr, c}
		}
	}
	c.events <- event{eventReady, c}
}

func (p *pool) purgeChild(c *child) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	delete(p.children, c.id)
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
	defer p.cond.L.Lock()

	current := len(p.children)

	for ; current < p.size; current++ {
		child, err := createChildFor(p)
		if err != nil {
			log.WithError(err).Error("can't create child; abort filling backlog")
			break
		}
		go func() {
			p.events <- event{eventCreated, child}
		}()
	}
}
