package node

import (
	"context"

	"github.com/boz/ephemerald/pubsub"
	"github.com/boz/ephemerald/types"
	dtypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
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
	return &eventPublisher{
		node:   node,
		bus:    bus,
		cancel: cancel,
		ctx:    ctx,
	}
}

type eventPublisher struct {
	node   Node
	bus    pubsub.Bus
	donech chan struct{}
	cancel context.CancelFunc
	ctx    context.Context
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
			cid := msg.Actor.Attributes[LabelEphemeraldContainerID]

			// TODO: err if empty

			ev := types.DockerEvent{
				Node:      ep.node.Host(),
				Pool:      types.ID(pid),
				Container: types.ID(cid),
				Message:   msg,
			}

			if err := ep.bus.Publish(ev); err != nil {
				// TODO: error
			}

		case <-errch:
			// TODO: retry
		}
	}
}
