package redis

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	rredis "github.com/garyburd/redigo/redis"
	"github.com/ovrclk/cleanroom"
)

const (
	defaultPort  = 6379
	defaultImage = "redis"
)

type Item struct {
	Cid      string `json:cid`
	Host     string `json:host`
	Port     string `json:port`
	Database string `json:database`
	URL      string `json:url`
}

type Pool interface {
	Checkout() (*Item, error)
	Return(*Item)
	WaitReady() error
	Stop() error
}

type Builder interface {
	WithDefaults() Builder
	WithImage(string) Builder
	WithSize(int) Builder
	WithLiveCheck(func(context.Context, *Item) error) Builder
	WithInitialize(func(context.Context, *Item) error) Builder
	WithReset(func(context.Context, *Item) error) Builder
	Create() (Pool, error)
}

type builder struct {
	size     int
	config   *cleanroom.Config
	pbuilder cleanroom.ProvisionerBuilder
}

func DefaultConfig() *cleanroom.Config {
	return applyDefaults(cleanroom.NewConfig())
}

func NewBuilder() Builder {
	return &builder{1, cleanroom.NewConfig(), cleanroom.BuildProvisioner()}
}

func DefaultBuilder() Builder {
	return NewBuilder().
		WithDefaults()
}

func (b *builder) WithDefaults() Builder {

	applyDefaults(b.config)

	b.pbuilder.WithLiveCheck(
		LiveCheck(
			cleanroom.LiveCheckDefaultTimeout,
			cleanroom.LiveCheckDefaultRetries,
			cleanroom.LiveCheckDefaultDelay,
			RediGoLiveCheck()))

	return b
}

func applyDefaults(config *cleanroom.Config) *cleanroom.Config {
	return config.
		WithImage(defaultImage).
		ExposePort("tcp", defaultPort)
}

func (b *builder) WithSize(size int) Builder {
	b.size = size
	return b
}

func (b *builder) WithImage(name string) Builder {
	b.config.WithImage(name)
	return b
}

func (b *builder) WithLiveCheck(fn func(context.Context, *Item) error) Builder {
	b.pbuilder.WithLiveCheck(func(ctx context.Context, si cleanroom.StatusItem) error {
		return fn(ctx, NewItem(si))
	})
	return b
}

func (b *builder) WithInitialize(fn func(context.Context, *Item) error) Builder {
	b.pbuilder.WithInitialize(func(ctx context.Context, si cleanroom.StatusItem) error {
		return fn(ctx, NewItem(si))
	})
	return b
}

func (b *builder) WithReset(fn func(context.Context, *Item) error) Builder {
	b.pbuilder.WithReset(func(ctx context.Context, si cleanroom.StatusItem) error {
		return fn(ctx, NewItem(si))
	})
	return b
}

func (b *builder) Create() (Pool, error) {
	return NewPool(b.config, b.size, b.pbuilder.Create())
}

func NewPool(config *cleanroom.Config, size int, provisioner cleanroom.Provisioner) (Pool, error) {
	parent, err := cleanroom.NewPool(config, size, provisioner)
	if err != nil {
		return nil, err
	}
	return &pool{parent}, nil
}

type pool struct {
	parent cleanroom.Pool
}

func (p *pool) Checkout() (*Item, error) {
	item, err := p.parent.Checkout()
	if err != nil {
		return nil, err
	}
	return NewItem(item), nil
}
func (p *pool) Return(item *Item) {
	p.parent.Return(item)
}
func (p *pool) WaitReady() error {
	return p.parent.WaitReady()
}
func (p *pool) Stop() error {
	return p.parent.Stop()
}

type item struct {
	parent cleanroom.StatusItem
	port   string
}

func NewItem(parent cleanroom.StatusItem) *Item {
	ports := cleanroom.TCPPortsFor(parent.Status())
	port := ports[strconv.Itoa(defaultPort)]

	item := &Item{
		Cid:      parent.ID(),
		Host:     "localhost",
		Port:     port,
		Database: "0",
	}

	item.URL = makeItemURL(item)

	return item
}

func (i *Item) ID() string {
	return i.Cid
}

func makeItemURL(i *Item) string {
	return fmt.Sprintf("redis://%v:%v/%v",
		url.QueryEscape(i.Host), url.QueryEscape(i.Port), url.QueryEscape(i.Database))
}

func LiveCheck(timeout time.Duration, tries int, delay time.Duration,
	fn func(context.Context, *Item) error) cleanroom.ProvisionFn {
	return cleanroom.LiveCheck(timeout, tries, delay,
		func(ctx context.Context, si cleanroom.StatusItem) error {
			return fn(ctx, NewItem(si))
		})
}

func RediGoLiveCheck() func(context.Context, *Item) error {
	return func(ctx context.Context, item *Item) error {

		conn, err := rredis.DialURL(item.URL)

		if err != nil {
			return err
		}
		_, err = conn.Do("PING")
		return err
	}
}
