package ui

import (
	"sync/atomic"
)

type peventId string

const (
	peventInit       peventId = "initializing"
	peventInitErr    peventId = "initialize-error"
	peventRunning    peventId = "running"
	peventDraining   peventId = "draining"
	peventDone       peventId = "done"
	peventNumItems   peventId = "num-items"
	peventNumPending peventId = "num-pending"
	peventNumReady   peventId = "num-ready"
)

type pevent struct {
	id       peventId
	poolName string
	err      error
	count    int
}

type ceventId string

const (
	ceventCreated   ceventId = "created"
	ceventStarted   ceventId = "started"
	ceventLive      ceventId = "live"
	ceventReady     ceventId = "ready"
	ceventResetting ceventId = "resetting"
	ceventExiting   ceventId = "exiting"
	ceventExited    ceventId = "exited"
	ceventAction    ceventId = "action-attempt"
	ceventResult    ceventId = "action-result"
)

type cevent struct {
	id          ceventId
	containerId string
	poolName    string

	lifecycleName  string
	actionName     string
	actionAttempt  int
	actionAttempts int

	err error
}

type processor struct {
	writer writer

	poolch      chan pevent
	containerch chan cevent

	pools      map[string]*pool
	containers map[string]*container

	stopped int32
	donech  chan bool
}

func newProcessor(w writer) *processor {
	p := &processor{
		writer: w,

		pools:      make(map[string]*pool),
		containers: make(map[string]*container),

		poolch:      make(chan pevent),
		containerch: make(chan cevent),
		donech:      make(chan bool),
	}

	go p.handlePoolEvents()
	go p.handleContainerEvents()

	return p
}

func (p *processor) stop() {
	if !atomic.CompareAndSwapInt32(&p.stopped, 0, 1) {
		return
	}
	close(p.donech)
	close(p.poolch)
	close(p.containerch)
	p.writer.stop()
}

func (p *processor) sendPoolEvent(e pevent) {
	if atomic.LoadInt32(&p.stopped) == 0 {
		select {
		case p.poolch <- e:
		case <-p.donech:
		}
	}
}

func (p *processor) sendContainerEvent(e cevent) {
	if atomic.LoadInt32(&p.stopped) == 0 {
		select {
		case p.containerch <- e:
		case <-p.donech:
		}
	}
}

func (p *processor) handlePoolEvents() {
	for e := range p.poolch {
		if pool, ok := p.pools[e.poolName]; ok {
			p.handlePoolUpdate(pool, e)
			continue
		}
		p.handlePoolCreate(e)
	}
}

func (p *processor) handleContainerEvents() {
	for e := range p.containerch {
		if c, ok := p.containers[e.containerId]; ok {
			p.handleContainerUpdate(c, e)
			continue
		}
		p.handleContainerCreate(e)
	}
}

func (p *processor) handleContainerUpdate(c *container, e cevent) {

	reset := false
	exited := false

	switch e.id {
	case ceventCreated:
		c.state = cstateCreated
		reset = true
	case ceventStarted:
		c.state = cstateStarted
		reset = true
	case ceventLive:
		c.state = cstateLive
	case ceventReady:
		c.state = cstateReady
		reset = true
	case ceventResetting:
		c.state = cstateResetting
		reset = true
	case ceventExiting:
		c.state = cstateExiting
	case ceventExited:
		c.state = cstateExited
		exited = true
	case ceventAction:
		c.lifecycleName = e.lifecycleName
		c.actionName = e.actionName
		c.actionAttempt = e.actionAttempt
		c.actionAttempts = e.actionAttempts
	case ceventResult:
		c.lifecycleName = e.lifecycleName
		c.actionName = e.actionName
		c.actionAttempt = e.actionAttempt
		c.actionAttempts = e.actionAttempts
		c.actionError = e.err
	}

	if reset {
		c.lifecycleName = ""
		c.actionName = ""
		c.actionAttempt = 0
		c.actionAttempts = 0
		c.actionError = nil
	}

	switch {
	case exited:
		p.writer.deleteContainer(*c)
		delete(p.containers, c.id)
	default:
		p.writer.updateContainer(*c)
	}
}

func (p *processor) handleContainerCreate(e cevent) {
	c := &container{
		id:    e.containerId,
		pname: e.poolName,
	}
	p.containers[c.id] = c
	p.handleContainerUpdate(c, e)
}

func (p *processor) handlePoolUpdate(pool *pool, e pevent) {

	switch e.id {
	case peventInit:
		pool.state = pstateInit
	case peventInitErr:
		pool.state = pstateErr
		pool.err = e.err
	case peventRunning:
		pool.state = pstateRunning
	case peventDraining:
		pool.state = pstateDraining
	case peventDone:
		pool.state = pstateStopped
	case peventNumItems:
		pool.numItems = e.count
	case peventNumPending:
		pool.numPending = e.count
	case peventNumReady:
		pool.numReady = e.count
	}

	p.writer.updatePool(*pool)
}

func (p *processor) handlePoolCreate(e pevent) {
	pool := &pool{
		name: e.poolName,
	}
	p.pools[pool.name] = pool
	p.handlePoolUpdate(pool, e)
}
