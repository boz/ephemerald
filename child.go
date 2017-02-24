package cleanroom

import (
	"context"
	"io"
	"os"
	"strconv"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type child struct {
	id      string
	shortId string

	status types.ContainerJSON

	ctx    context.Context
	cancel context.CancelFunc

	pool   *pool
	client *client.Client
	done   chan interface{}

	log logrus.FieldLogger
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
		id:      cid,
		shortId: cid[0:12],

		ctx:    ctx,
		cancel: cancel,

		pool:   p,
		client: p.client,
		done:   make(chan interface{}),
		log:    lcid(p.log, cid).WithField("component", "child"),
	}

	go c.dumpLogs()

	go c.monitor()

	return c, nil
}

func (c *child) doStart() {

	if err := c.start(); err != nil {
		c.sendEvent(eventStartFailed)
		return
	}

	status, err := c.getStatus()
	if err != nil {
		c.sendEvent(eventStartFailed)
		return
	}

	c.status = status

	c.sendEvent(eventStarted)
}

func (c *child) start() error {
	options := types.ContainerStartOptions{}
	if err := c.client.ContainerStart(c.ctx, c.id, options); err != nil {
		c.log.WithError(err).Error("error starting container")
		return err
	}
	return nil
}

func (c *child) getStatus() (types.ContainerJSON, error) {
	status, err := c.client.ContainerInspect(c.ctx, c.id)
	if err != nil {
		c.log.WithError(err).Error("error inspecting container")
	}
	return status, err
}

func (c *child) kill() error {
	err := c.client.ContainerKill(c.ctx, c.id, "KILL")
	if err != nil {
		c.log.WithError(err).Error("error killing container")
	}
	return err
}

func (c *child) monitor() {
	err := c.doMonitor()
	for err != nil && c.ctx.Err() == nil {
		c.log.WithError(err).Errorf("error reading events")
		err = c.doMonitor()
	}
	c.log.Debugf("done monitoring")
}

func (c *child) doMonitor() error {
	f := filters.NewArgs()
	f.Add("type", "container")
	f.Add("container", c.id)

	options := types.EventsOptions{
		Filters: f,
	}

	eventq, errq := c.client.Events(c.ctx, options)

	status := syscall.WaitStatus(0)

	for {
		select {
		case evt := <-eventq:
			c.log.Debugf("docker event: %v", evt.Status)
			switch evt.Status {
			case "die":
				if v, ok := evt.Actor.Attributes["exitCode"]; ok {
					if code, err := strconv.Atoi(v); err != nil {
						c.log.WithError(err).Error("error converting exit code")
					} else {
						status = syscall.WaitStatus(code)
					}
				}
			case "detach", "destroy":
				c.log.Debugf("container exited")
				if status == 0 {
					c.sendEvent(eventExitSuccess)
				} else {
					c.sendEvent(eventExitError)
				}
			}
		case err := <-errq:
			if err == io.EOF {
				c.log.Debugf("done reading docker events")
				return nil
			}
			return err
		}
	}
}

func (c *child) dumpLogs() {

	c.log.Debug("dumping logs")

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Details:    true,
		Tail:       "1",
	}

	body, err := c.client.ContainerLogs(c.ctx, c.id, options)
	if err != nil {
		c.log.WithError(err).Error("error getting logs")
	}
	defer body.Close()

	_, err = io.Copy(os.Stdout, body)
	if err != nil {
		c.log.WithError(err).Error("reading logs")
	}

}

func (c *child) sendEvent(id eventId) {
	c.pool.sendEvent(id, c)
}
