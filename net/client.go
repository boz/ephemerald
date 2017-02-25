package net

import (
	"fmt"

	"github.com/koding/kite"
	"github.com/ovrclk/cleanroom/builtin/pg"
	"github.com/ovrclk/cleanroom/builtin/redis"
)

type ClientBuilder struct {
	kclient *kite.Client

	pgb *pg.ClientBuilder
}

type Client struct {
	kclient *kite.Client

	redis *redis.Client
	pg    *pg.Client
}

func NewClientBuilder() *ClientBuilder {
	k := kite.New(kiteName, kiteVersion)
	k.SetLogLevel(kite.DEBUG)
	c := k.NewClient("")

	pgb := pg.NewClientBuilder().WithClient(c)

	return &ClientBuilder{c, pgb}
}

func (b *ClientBuilder) WithHost(host string) *ClientBuilder {
	b.kclient.URL = fmt.Sprintf("http://%v:%v/kite", host, kitePort)
	b.kclient.Environment = host
	return b
}

func (b *ClientBuilder) BuildPG(fn func(*pg.ClientBuilder)) *ClientBuilder {
	fn(b.pgb)
	return b
}

func (b *ClientBuilder) Create() (*Client, error) {
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
