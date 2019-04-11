package node

import (
	"context"

	"github.com/boz/ephemerald/pubsub"
	"github.com/boz/ephemerald/types"
	dtypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/ovrclk/akash/provider/event"
)

type EventPublisher interface {
	Node() Node
	Stop()
	Done() <-chan struct{}
}

func NewEventPublisher(ctx context.Context, node Node, bus event.Bus) EventPublisher {
	return nil
}

type eventPublisher struct {
	node Node
	bus  pubsub.Bus
	ctx  context.Context
}

const (
	LabelEphemeraldPoolID      = "ephemerald.io.pool-id"
	LabelEphemeraldContainerID = "ephemerald.io.container-id"
)

func (ep *eventPublisher) run() {
	ctx, cancel := context.WithCancel(ep.ctx)
	defer cancel()

	filters := filters.NewArgs()
	filters.Add("type", "container")
	filters.Add("label", LabelEphemeraldPoolID+"=")
	filters.Add("label", LabelEphemeraldContainerID+"=")

	opts := dtypes.EventsOptions{
		Filters: filters,
	}

	msgch, errch := ep.node.Client().Events(ctx, opts)

	for {
		select {
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

			if err := ep.bus.Publish(ctx, ev); err != nil {
				// TODO: error
			}

		case err := <-errch:
			// TODO:
		}
	}
}
