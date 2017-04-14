package cleanroom

import (
	"io"
	"os"
	"strconv"
	"sync"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
)

type PoolContainer interface {
	StatusItem
	Start()
	Stop()
	Events() <-chan containerEvent
}

type containerEvent string

const (
	containerEventStarted     containerEvent = "started"
	containerEventStartFailed containerEvent = "start-failed"
	containerEventExitSuccess containerEvent = "exit-success"
	containerEventExitError   containerEvent = "exit-error"
)

type poolContainer struct {
	adapter Adapter

	id string

	status types.ContainerJSON

	events chan containerEvent

	done chan interface{}

	startOnce sync.Once
	stopOnce  sync.Once

	log logrus.FieldLogger
}

func createPoolContainer(log logrus.FieldLogger, adapter Adapter) (*poolContainer, error) {
	log = log.WithField("component", "pool-container")

	cid, err := adapter.CreateContainer()
	if err != nil {
		log.WithError(err).
			Error("can't create container")
		return nil, err
	}

	c := &poolContainer{
		adapter: adapter,

		id: cid,

		events: make(chan containerEvent),

		done: make(chan interface{}),
		log:  log,
	}

	go c.dumpLogs()

	go c.monitor()

	return c, nil
}

func (c *poolContainer) ID() string {
	return c.id
}

func (c *poolContainer) Status() types.ContainerJSON {
	return c.status
}

func (c *poolContainer) Events() <-chan containerEvent {
	return c.events
}

func (c *poolContainer) Start() {
	c.startOnce.Do(func() {
		go c.doStart()
	})
}

func (c *poolContainer) Stop() {
	c.stopOnce.Do(func() {
		go c.doStop()
	})
}

func (c *poolContainer) doStart() {
	if err := c.start(); err != nil {
		c.events <- containerEventStartFailed
		return
	}

	status, err := c.getStatus()
	if err != nil {
		c.events <- containerEventStartFailed
		return
	}

	c.status = status

	c.events <- containerEventStarted
}

func (c *poolContainer) doStop() {
	err := c.adapter.ContainerKill(c.id, "KILL")
	if err != nil {
		c.log.WithError(err).Error("error killing container")
	}
}

func (c *poolContainer) start() error {
	options := types.ContainerStartOptions{}
	if err := c.adapter.ContainerStart(c.id, options); err != nil {
		c.log.WithError(err).Error("error starting container")
		return err
	}
	return nil
}

func (c *poolContainer) getStatus() (types.ContainerJSON, error) {
	status, err := c.adapter.ContainerInspect(c.id)
	if err != nil {
		c.log.WithError(err).Error("error inspecting container")
	}
	return status, err
}

func (c *poolContainer) monitor() {
	err := c.doMonitor()
	for err != nil {
		c.log.WithError(err).Errorf("error reading events")
		err = c.doMonitor()
	}
	c.log.Debugf("done monitoring")
}

func (c *poolContainer) doMonitor() error {
	f := filters.NewArgs()
	f.Add("type", "container")
	f.Add("container", c.id)

	options := types.EventsOptions{
		Filters: f,
	}

	eventq, errq := c.adapter.Events(options)

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
					c.events <- containerEventExitSuccess
				} else {
					c.events <- containerEventExitError
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

func (c *poolContainer) dumpLogs() {

	c.log.Debug("dumping logs")

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Details:    true,
		Tail:       "1",
	}

	body, err := c.adapter.ContainerLogs(c.id, options)
	if err != nil {
		c.log.WithError(err).Error("error getting logs")
	}
	defer body.Close()

	_, err = io.Copy(os.Stdout, body)
	if err != nil {
		c.log.WithError(err).Error("reading logs")
	}

	c.log.Debug("done dumping logs")
}
