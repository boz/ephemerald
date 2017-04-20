package ephemerald

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

type poolContainer interface {
	StatusItem
	start()
	stop()
	events() <-chan containerEvent
}

type containerEvent string

const (
	containerEventStarted     containerEvent = "started"
	containerEventStartFailed containerEvent = "start-failed"
	containerEventExitSuccess containerEvent = "exit-success"
	containerEventExitError   containerEvent = "exit-error"
)

type pcontainer struct {
	adapter dockerAdapter

	id string

	status types.ContainerJSON

	eventch chan containerEvent

	done chan interface{}

	startOnce sync.Once
	stopOnce  sync.Once

	log logrus.FieldLogger
}

func createPoolContainer(log logrus.FieldLogger, adapter dockerAdapter) (poolContainer, error) {
	log = log.WithField("component", "pool-container")

	cid, err := adapter.createContainer()
	if err != nil {
		log.WithError(err).
			Error("can't create container")
		return nil, err
	}

	c := &pcontainer{
		adapter: adapter,

		id: cid,

		eventch: make(chan containerEvent),

		done: make(chan interface{}),
		log:  lcid(log, cid),
	}

	go c.dumpLogs()

	go c.monitor()

	return c, nil
}

func (c *pcontainer) ID() string {
	return c.id
}

func (c *pcontainer) Status() types.ContainerJSON {
	return c.status
}

func (c *pcontainer) events() <-chan containerEvent {
	return c.eventch
}

func (c *pcontainer) start() {
	c.startOnce.Do(func() {
		go c.doStart()
	})
}

func (c *pcontainer) stop() {
	c.stopOnce.Do(func() {
		go c.doStop()
	})
}

func (c *pcontainer) doStart() {
	if err := c.startContainer(); err != nil {
		c.eventch <- containerEventStartFailed
		return
	}

	status, err := c.getStatus()
	if err != nil {
		c.eventch <- containerEventStartFailed
		return
	}

	c.status = status

	c.eventch <- containerEventStarted
}

func (c *pcontainer) doStop() {
	err := c.adapter.containerKill(c.id, "KILL")
	if err != nil {
		c.log.WithError(err).Error("error killing container")
	}
}

func (c *pcontainer) startContainer() error {
	options := types.ContainerStartOptions{}
	if err := c.adapter.containerStart(c.id, options); err != nil {
		c.log.WithError(err).Error("error starting container")
		return err
	}
	return nil
}

func (c *pcontainer) getStatus() (types.ContainerJSON, error) {
	status, err := c.adapter.containerInspect(c.id)
	if err != nil {
		c.log.WithError(err).Error("error inspecting container")
	}
	return status, err
}

func (c *pcontainer) monitor() {
	err := c.doMonitor()
	for err != nil {
		c.log.WithError(err).Errorf("error reading events")
		err = c.doMonitor()
	}
	c.log.Debugf("done monitoring")
}

func (c *pcontainer) doMonitor() error {
	f := filters.NewArgs()
	f.Add("type", "container")
	f.Add("container", c.id)

	options := types.EventsOptions{
		Filters: f,
	}

	eventq, errq := c.adapter.containerEvents(options)

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
					c.eventch <- containerEventExitSuccess
				} else {
					c.eventch <- containerEventExitError
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

func (c *pcontainer) dumpLogs() {

	c.log.Debug("dumping logs")

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Details:    true,
		Tail:       "1",
	}

	body, err := c.adapter.containerLogs(c.id, options)
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
