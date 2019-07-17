package node

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type Node interface {
	Host() string
	Client() *client.Client
	Endpoint() string
}

func NewFromEnv(ctx context.Context) (Node, error) {
	client, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	ping, err := client.Ping(ctx)
	if err != nil {
		return nil, err
	}

	return &node{
		endpoint: "127.0.0.1",
		client:   client,
		ping:     ping,
	}, nil
}

type node struct {
	endpoint string
	client   *client.Client
	ping     types.Ping
}

func (n *node) Host() string {
	return n.client.DaemonHost()
}

func (n *node) Client() *client.Client {
	return n.client
}

func (n *node) Endpoint() string {
	return n.endpoint
}
