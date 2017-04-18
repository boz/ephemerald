package net

import (
	"fmt"

	"github.com/boz/ephemerald/builtin/pg"
	"github.com/boz/ephemerald/builtin/redis"
	"github.com/koding/kite"
)

type ClientBuilder struct {
	kclient *kite.Client
	kite    *kite.Kite

	host string
	port int

	pgb *pg.ClientBuilder
}

type Client struct {
	kclient *kite.Client

	redis *redis.Client
	pg    *pg.Client
}

func NewClientBuilder() *ClientBuilder {
	k := kite.New(kiteName+"-client", kiteVersion)
	c := k.NewClient("")
	c.Concurrent = true
	c.ConcurrentCallbacks = true

	pgb := pg.NewClientBuilder().WithClient(c)

	return &ClientBuilder{c, k, "localhost", DefaultPort, pgb}
}

func (b *ClientBuilder) WithHost(host string) *ClientBuilder {
	b.host = host
	return b
}

func (b *ClientBuilder) WithPort(port int) *ClientBuilder {
	b.port = port
	return b
}

func (b *ClientBuilder) PG() *pg.ClientBuilder {
	return b.pgb
}

func (b *ClientBuilder) Create() (*Client, error) {
	b.kclient.URL = fmt.Sprintf("http://%v:%v/kite", b.host, b.port)
	b.kite.Config.Environment = b.host

	pg, _ := b.pgb.Create()
	redis := redis.BuildClient(b.kclient)

	if err := b.kclient.Dial(); err != nil {
		return nil, err
	}
	return &Client{b.kclient, redis, pg}, nil
}

func (c *Client) Redis() *redis.Client {
	return c.redis
}

func (c *Client) PG() *pg.Client {
	return c.pg
}
