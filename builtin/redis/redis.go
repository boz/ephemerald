package redis

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	rredis "github.com/garyburd/redigo/redis"
	"github.com/boz/ephemerald"
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
	config   *ephemerald.Config
	pbuilder ephemerald.ProvisionerBuilder
}

func DefaultConfig() *ephemerald.Config {
	return applyDefaults(ephemerald.NewConfig())
}

func NewBuilder() Builder {
	return &builder{1, ephemerald.NewConfig(), ephemerald.BuildProvisioner()}
}

func DefaultBuilder() Builder {
	return NewBuilder().
		WithDefaults()
}

func (b *builder) WithDefaults() Builder {

	applyDefaults(b.config)

	b.pbuilder.WithLiveCheck(
		LiveCheck(
			ephemerald.LiveCheckDefaultTimeout,
			ephemerald.LiveCheckDefaultRetries,
			ephemerald.LiveCheckDefaultDelay,
			RediGoLiveCheck()))

	return b
}

func applyDefaults(config *ephemerald.Config) *ephemerald.Config {
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
	b.pbuilder.WithLiveCheck(func(ctx context.Context, si ephemerald.StatusItem) error {
		return fn(ctx, NewItem(si))
	})
	return b
}

func (b *builder) WithInitialize(fn func(context.Context, *Item) error) Builder {
	b.pbuilder.WithInitialize(func(ctx context.Context, si ephemerald.StatusItem) error {
		return fn(ctx, NewItem(si))
	})
	return b
}

func (b *builder) WithReset(fn func(context.Context, *Item) error) Builder {
	b.pbuilder.WithReset(func(ctx context.Context, si ephemerald.StatusItem) error {
		return fn(ctx, NewItem(si))
	})
	return b
}

func (b *builder) Create() (Pool, error) {
	return NewPool(b.config, b.size, b.pbuilder.Create())
}

func NewPool(config *ephemerald.Config, size int, provisioner ephemerald.Provisioner) (Pool, error) {
	parent, err := ephemerald.NewPool(config, size, provisioner)
	if err != nil {
		return nil, err
	}
	return &pool{parent}, nil
}

type pool struct {
	parent ephemerald.Pool
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
	parent ephemerald.StatusItem
	port   string
}

func NewItem(parent ephemerald.StatusItem) *Item {
	ports := ephemerald.TCPPortsFor(parent.Status())
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
	fn func(context.Context, *Item) error) ephemerald.ProvisionFn {
	return ephemerald.LiveCheck(timeout, tries, delay,
		func(ctx context.Context, si ephemerald.StatusItem) error {
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
