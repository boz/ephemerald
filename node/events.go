package node

import (
	"context"

	"github.com/boz/ephemerald/log"
	"github.com/boz/ephemerald/pubsub"
	"github.com/boz/ephemerald/types"
	dtypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/sirupsen/logrus"
)

const (
	LabelEphemeraldPoolID      = "ephemerald.io.pool-id"
	LabelEphemeraldContainerID = "ephemerald.io.container-id"
)

type EventPublisher interface {
	Node() Node
	Stop()
	Done() <-chan struct{}
}

func NewEventPublisher(ctx context.Context, node Node, bus pubsub.Bus) EventPublisher {
	ctx, cancel := context.WithCancel(ctx)
	ep := &eventPublisher{
		node:   node,
		bus:    bus,
		donech: make(chan struct{}),
		cancel: cancel,
		ctx:    ctx,
		log:    log.FromContext(ctx).WithField("cmp", "node/event-publisher"),
	}

	go ep.run()

	return ep
}

type eventPublisher struct {
	node   Node
	bus    pubsub.Bus
	donech chan struct{}
	cancel context.CancelFunc
	ctx    context.Context
	log    logrus.FieldLogger
}

func (ep *eventPublisher) Node() Node {
	return ep.node
}

func (ep *eventPublisher) Stop() {
	ep.cancel()
}

func (ep *eventPublisher) Done() <-chan struct{} {
	return ep.donech
}

func (ep *eventPublisher) run() {
	defer close(ep.donech)
	defer ep.cancel()

	filters := filters.NewArgs()
	filters.Add("type", "container")
	filters.Add("label", LabelEphemeraldPoolID+"=")
	filters.Add("label", LabelEphemeraldContainerID+"=")

	opts := dtypes.EventsOptions{
		Filters: filters,
	}

	msgch, errch := ep.node.Client().Events(ep.ctx, opts)

	for {
		select {
		case <-ep.ctx.Done():
			return
		case msg := <-msgch:
			pid := msg.Actor.Attributes[LabelEphemeraldPoolID]
			iid := msg.Actor.Attributes[LabelEphemeraldContainerID]

			// TODO: err if empty

			ev := types.DockerEvent{
				Node:     ep.node.Host(),
				Pool:     types.ID(pid),
				Instance: types.ID(iid),
				Message:  msg,
			}

			if err := ep.bus.Publish(ev); err != nil {
				ep.log.WithError(err).Warn("bus publish")
				return
			}

		case err := <-errch:
			// TODO: retry
			ep.log.WithError(err).Warn("docker events")
			return
		}
	}
}
