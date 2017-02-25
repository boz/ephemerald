package pg

import (
	"context"
	"fmt"

	"github.com/koding/kite"
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
	kclient    *kite.Client
	initialize ProvisionFn
	reset      ProvisionFn
}

type Client struct {
	kclient *kite.Client
}

func NewClientBuilder() *ClientBuilder {
	return &ClientBuilder{}
}

func (b *ClientBuilder) WithClient(c *kite.Client) *ClientBuilder {
	b.kclient = c
	return b
}

func (b *ClientBuilder) WithInitialize(fn ProvisionFn) *ClientBuilder {
	b.initialize = fn
	return b
}

func (b *ClientBuilder) WithReset(fn ProvisionFn) *ClientBuilder {
	b.reset = fn
	return b
}

func (b *ClientBuilder) Create() (*Client, error) {

	if b.kclient == nil {
		return nil, errClientRequired
	}

	c := &Client{kclient: b.kclient}
	c.kclient.LocalKite.HandleFunc(rpcInitializeName, c.makeProvisioner(b.initialize))
	c.kclient.LocalKite.HandleFunc(rpcResetName, c.makeProvisioner(b.reset))

	return c, nil
}

func (c *Client) Checkout() (*Item, error) {
	response, err := c.kclient.Tell(rpcCheckoutName)
	if err != nil {
		return nil, err
	}
	i := Item{}
	response.MustUnmarshal(&i)
	return &i, nil
}

func (c *Client) Return(i *Item) error {
	_, err := c.kclient.Tell(rpcReturnName, i)
	return err
}

func (c *Client) makeProvisioner(fn ProvisionFn) kite.HandlerFunc {
	return func(r *kite.Request) (interface{}, error) {
		if fn == nil {
			return nil, nil
		}
		i := Item{}
		r.Args.One().MustUnmarshal(&i)
		return nil, fn(context.TODO(), &i)
	}
}
