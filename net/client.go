package net

import (
	"fmt"

	"github.com/boz/ephemerald/params"
	"github.com/koding/kite"
)

type ClientBuilder struct {
	kclient *kite.Client
	kite    *kite.Kite

	host string
	port int
}

type Client struct {
	kclient *kite.Client
}

func NewClientBuilder() *ClientBuilder {
	k := kite.New(kiteName+"-client", kiteVersion)
	c := k.NewClient("")
	// XXX: race condition
	//k.SetLogLevel(kite.DEBUG)
	c.Concurrent = true
	c.ConcurrentCallbacks = true
	return &ClientBuilder{c, k, "localhost", DefaultPort}
}

func (b *ClientBuilder) WithHost(host string) *ClientBuilder {
	b.host = host
	return b
}

func (b *ClientBuilder) WithPort(port int) *ClientBuilder {
	b.port = port
	return b
}

func (b *ClientBuilder) Create() (*Client, error) {
	b.kclient.URL = fmt.Sprintf("http://%v:%v/kite", b.host, b.port)
	b.kite.Config.Environment = b.host

	if err := b.kclient.Dial(); err != nil {
		return nil, err
	}
	return &Client{b.kclient}, nil
}

func (c *Client) Checkout(names ...string) (params.Set, error) {
	ps := params.Set{}
	response, err := c.kclient.Tell(rpcCheckoutName, names)
	if err != nil {
		return ps, err
	}
	response.MustUnmarshal(&ps)
	return ps, nil
}

func (c *Client) Return(ps params.Set) error {
	_, err := c.kclient.Tell(rpcReturnName, ps)
	return err
}
