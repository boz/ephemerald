package pg

import (
	"fmt"

	"github.com/koding/kite"
	"github.com/ovrclk/cleanroom"
)

const (
	rpcInitializeName = "pg.initialize"
	rpcResetName      = "pg.reset"
	rpcCheckoutName   = "pg.checkout"
	rpcReturnName     = "pg.return"
)

var (
	errClientRequired = fmt.Errorf("pg/client-builder: client required")
)

type ClientBuilder struct {
	client     *kite.Client
	initialize ProvisionFn
	reset      ProvisionFn
}

func NewClientBuilder() *ClientBuilder {
	return &ClientBuilder{
		pbuilder: cleanroom.BuildProvisioner(),
	}
}

func (b *ClientBuilder) WithClient(client *kite.Client) ClientBuilder {
	b.client = client
	return b
}

func (b *ClientBuilder) WithInitialize(fn ProvisionFn) ClientBuilder {
	b.initialize = fn
	return b
}

func (b *ClientBuilder) WithReset(fn ProvisionFn) ClientBuilder {
	b.reset = fn
	return b
}

func (b *ClientBuilder) Create() (*Client, error) {

	if b.client == nil {
		return nil, errClientRequired
	}

	b.client.LocalKite.HandlerFunc(rpcInitializeName, MakeRPCProvisioner(b.initialize))
	b.client.LocalKite.HandlerFunc(rpcResetName, MakeRPCProvisioner(b.reset))

	return &Client{client: b.client}, nil
}

func (c *Client) Checkout() (*Item, error) {
	response := c.client.Tell(rpcCheckoutName, item)
	if response.Err != nil {
		return nil, response.Err
	}
	return unmarshalPartial(response.Result)
}

func (c *Client) Return(*Item) error {
	response := c.client.Tell(rpcCheckoutName, item)
	return response.Err
}

func unmarshalPartial(p kite.Partial) (*Item, error) {
	item := Item{}
	return &item, p.One().Unmarshal(&item)
}

func MakeRPCProvisioner(fn ProvisionFn) kite.HandlerFunc {
	return func(r *kite.Request) (interface{}, error) {
		if fn == nil {
			return nil, nil
		}
		item := Item{}
		err := r.Args.One().Unmarshal(&item)
		if err != nil {
			return nil, err
		}
		return err, fn(item)
	}
}
