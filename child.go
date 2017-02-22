package cpool

import (
	"context"
	"strconv"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type child struct {
	id string

	status types.ContainerJSON

	ctx    context.Context
	cancel context.CancelFunc

	client *client.Client
	events chan<- event
	done   chan interface{}
}

func (c *child) ID() string {
	return c.id
}

func (c *child) Status() types.ContainerJSON {
	return c.status
}

func createChildFor(p *pool) (*child, error) {
	cid, err := createContainer(p, p.ref, p.config)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(p.ctx)

	c := &child{
		id: cid,

		ctx:    ctx,
		cancel: cancel,

		client: p.client,
		events: p.events,
		done:   make(chan interface{}),
	}

	go c.monitor()

	return c, nil
}

func (c *child) doStart() {
	if err := c.start(); err != nil {
		c.events <- event{eventStartFailed, c}
		return
	}

	status, err := c.getStatus()
	if err != nil {
		c.events <- event{eventStartFailed, c}
		return
	}

	c.status = status

	c.events <- event{eventStarted, c}
}

func (c *child) start() error {
	options := types.ContainerStartOptions{}
	if err := c.client.ContainerStart(c.ctx, c.id, options); err != nil {
		return err
	}
	return nil
}

func (c *child) getStatus() (types.ContainerJSON, error) {
	status, err := c.client.ContainerInspect(c.ctx, c.id)
	return status, err
}

func (c *child) kill() error {
	return c.client.ContainerKill(c.ctx, c.id, "SIGKILL")
}

func (c *child) monitor() {
	f := filters.NewArgs()
	f.Add("type", "container")
	f.Add("container", c.id)

	options := types.EventsOptions{
		Filters: f,
	}

	eventq, errq := c.client.Events(c.ctx, options)

	status := 125

	for {
		select {
		case <-c.ctx.Done():
			log.Errorf("context deadline exceeded")
			return
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
				if status == 0 {
					c.events <- event{eventExitSuccess, c}
				} else {
					c.events <- event{eventExitError, c}
				}
			}
		case err := <-errq:
			log.WithError(err).Error("error running container")
		}
	}
}
