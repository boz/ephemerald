package cpool

import (
	"context"
	"io"
	"strconv"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/registry"
	"github.com/docker/go-connections/nat"
)

var (
	log = logrus.StandardLogger().
		WithField("package", "github.com/ovrclk/resource-pool").
		WithField("module", "pool")
)

type Pool interface {
	Fetch() error
	Stop() error

	Checkout() Item
	Return(Item)
}

type Resource interface {
	Config() *Config
	Create() Item
}

type Item interface{}

type pool struct {
	r    Resource
	size int

	client *client.Client

	cfg  *Config
	ref  reference.Named
	info *registry.RepositoryInfo

	ctx    context.Context
	cancel context.CancelFunc

	events chan (event)

	cond *sync.Cond

	ready map[string]*rcontainer
	taken map[string]*rcontainer

	containers map[string]*rcontainer
}

type eventId int

const (
	eventCreated eventId = iota
	eventStarted
	eventDied
	eventReturned
	eventCheckedOut
)

type event struct {
	id        eventId
	container *rcontainer
}

type cState int

const (
	cStateCreated cState = iota
	cStateStarted
	cStateStopped
)

type rcontainer struct {
	id    string
	state cState

	ctx    context.Context
	cancel context.CancelFunc

	client *client.Client

	donech <-chan int
}

func NewPool(r Resource, size int) (Pool, error) {
	cfg := r.Config()

	ref, err := reference.ParseNormalizedNamed(cfg.Image)
	if err != nil {
		log.Errorf("Unable to parse image '%s'", cfg.Image)
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
		r:      r,
		size:   size,
		cfg:    cfg,
		ref:    ref,
		info:   info,
		client: client,
		ctx:    ctx,
		cancel: cancel,
		events: events,
		cond:   sync.NewCond(&sync.Mutex{}),
	}
	go p.run()

	return p, nil
}

func (p *pool) Stop() error {
	p.cancel()
	return nil
}

func (p *pool) Checkout() Item {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()

	for len(p.ready) == 0 {
		p.cond.Wait()
	}

	next := p.ready[0]
	p.ready = p.ready[1:]

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
	exists, err := p.imageExists()
	if err != nil {
		return err
	}
	if exists {
		log.Infof("found image '%s' (%s)", reference.FamiliarString(p.ref), p.ref.Name())
		return nil
	}

	log.Infof(
		"Unable to find image '%s' (%s) locally.  Pulling...",
		reference.FamiliarString(p.ref), p.ref.Name())

	return p.pullImage(p.ref)
}

func (p *pool) imageExists() (bool, error) {
	_, _, err := p.client.ImageInspectWithRaw(p.ctx, p.ref.Name())
	switch {
	case err == nil:
		return true, nil
	case client.IsErrImageNotFound(err):
		return false, nil
	default:
		return false, err
	}
}

func (p *pool) pullImage(ref reference.Named) error {
	log.Infof("pulling image '%s'", ref.String())
	body, err := p.client.ImageCreate(p.ctx, ref.String(), types.ImageCreateOptions{})
	if err != nil {
		return err
	}
	defer body.Close()

	for {
		buf := make([]byte, 1024)
		_, err := body.Read(buf)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *pool) run() {
	for {
		p.primeBacklog()
		select {
		case <-p.ctx.Done():
			return
		case e := <-p.events:
			switch e.id {
			case eventCreated:
			case eventStarted:
			case eventDied:
			case eventCheckedOut:
			case eventReturned:
			}
		}
	}
}

func (p *pool) primeBacklog() {
	p.cond.L.Lock()
	defer p.cond.L.Lock()

	current := len(p.taken) + len(p.pending) + len(p.ready)

	for ; current < p.size; current++ {
		p.createItem()
	}
}

func (p *pool) createItem() (Item, error) {
	rc, err := p.createContainer()
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case <-rc.ctx.Done():
				rc.kill()
			}
		}
	}()
	return nil, nil
}
func (p *pool) createContainer() (*rcontainer, error) {

	config := &container.Config{
		Image:        p.ref.Name(),
		Cmd:          p.cfg.Cmd,
		Env:          p.cfg.Env,
		Volumes:      p.cfg.Volumes,
		AttachStdin:  false,
		AttachStdout: false,
		AttachStderr: false,
		ExposedPorts: make(nat.PortSet),
	}

	for p, _ := range p.cfg.Ports {
		config.ExposedPorts[p] = struct{}{}
	}

	hconfig := &container.HostConfig{
		AutoRemove:      true,
		PublishAllPorts: true,
		RestartPolicy:   container.RestartPolicy{},
	}

	nconfig := &network.NetworkingConfig{}

	name := ""

	cnt, err := p.client.ContainerCreate(p.ctx, config, hconfig, nconfig, name)
	if err != nil {
		log.WithError(err).Error("can't create container")
		return nil, err
	}

	log.Infof("Created container %v", cnt.ID)
	for _, w := range cnt.Warnings {
		log.Warn(w)
	}

	ctx, cancel := context.WithCancel(p.ctx)
	rc := &rcontainer{id: cnt.ID, ctx: ctx, cancel: cancel, client: p.client}

	rc.waitExit()
	rc.start()

	return rc, nil
}

func (rc *rcontainer) start() error {
	options := types.ContainerStartOptions{}
	if err := rc.client.ContainerStart(rc.ctx, rc.id, options); err != nil {
		rc.kill()
		return err
	}
	return nil
}

func (rc *rcontainer) kill() error {
	return rc.client.ContainerKill(rc.ctx, rc.id, "SIGKILL")
}

func (rc *rcontainer) waitExit() {
	f := filters.NewArgs()
	f.Add("type", "container")
	f.Add("container", rc.id)

	options := types.EventsOptions{
		Filters: f,
	}

	eventq, errq := rc.client.Events(rc.ctx, options)

	donech := make(chan int)
	status := 125

	go func() {
		defer func() {
			donech <- status
		}()

		for {
			select {
			case evt := <-eventq:
				log.Debugf("event: %+v", evt)
				switch evt.Status {
				case "die":
					if v, ok := evt.Actor.Attributes["exitCode"]; ok {
						if code, err := strconv.Atoi(v); err != nil {
							log.WithError(err).Error("error converting exit code")
						} else {
							status = code
						}
					}
				case "detach", "destroy":
					return
				}
			case err := <-errq:
				log.WithError(err).Error("error running container")
				return
			}
		}
	}()

	rc.donech = donech
}
