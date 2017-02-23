package redis

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	rredis "github.com/garyburd/redigo/redis"
	"github.com/ovrclk/cpool"
)

const (
	defaultPort  = 6379
	defaultImage = "redis"
)

type Item interface {
	ID() string

	Host() string
	Port() string
	Database() string
	URL() string
}

type Pool interface {
	Checkout() Item
	Return(Item)
	Stop() error
}

type Builder interface {
	WithDefaults() Builder
	WithImage(string) Builder
	WithSize(int) Builder
	WithLiveCheck(func(context.Context, Item) error) Builder
	WithInitialize(func(context.Context, Item) error) Builder
	WithReset(func(context.Context, Item) error) Builder
	Create() (Pool, error)
}

type builder struct {
	size     int
	config   *cpool.Config
	pbuilder cpool.ProvisionerBuilder
}

func DefaultConfig() *cpool.Config {
	return cpool.NewConfig().
		WithImage(defaultImage).
		ExposePort("tcp", defaultPort)
}

func NewBuilder() Builder {
	return &builder{1, cpool.NewConfig(), cpool.BuildProvisioner()}
}

func DefaultBuilder() Builder {
	return NewBuilder().
		WithDefaults()
}

func (b *builder) WithDefaults() Builder {
	b.config.WithImage(defaultImage).
		ExposePort("tcp", defaultPort)

	b.pbuilder.WithLiveCheck(
		LiveCheck(
			cpool.LiveCheckDefaultTimeout,
			cpool.LiveCheckDefaultRetries,
			cpool.LiveCheckDefaultDelay,
			RediGoLiveCheck()))

	return b
}

func (b *builder) WithSize(size int) Builder {
	b.size = size
	return b
}

func (b *builder) WithImage(name string) Builder {
	b.config.WithImage(name)
	return b
}

func (b *builder) WithLiveCheck(fn func(context.Context, Item) error) Builder {
	b.pbuilder.WithLiveCheck(func(ctx context.Context, si cpool.StatusItem) error {
		return fn(ctx, NewItem(si))
	})
	return b
}

func (b *builder) WithInitialize(fn func(context.Context, Item) error) Builder {
	b.pbuilder.WithInitialize(func(ctx context.Context, si cpool.StatusItem) error {
		return fn(ctx, NewItem(si))
	})
	return b
}

func (b *builder) WithReset(fn func(context.Context, Item) error) Builder {
	b.pbuilder.WithReset(func(ctx context.Context, si cpool.StatusItem) error {
		return fn(ctx, NewItem(si))
	})
	return b
}

func (b *builder) Create() (Pool, error) {
	return NewPool(b.config, b.size, b.pbuilder.Create())
}

func NewPool(config *cpool.Config, size int, provisioner cpool.Provisioner) (Pool, error) {
	parent, err := cpool.NewPool(config, size, provisioner)
	if err != nil {
		return nil, err
	}
	return &pool{parent}, nil
}

type pool struct {
	parent cpool.Pool
}

func (p *pool) Checkout() Item {
	return NewItem(p.parent.Checkout())
}
func (p *pool) Return(item Item) {
	p.parent.Return(item)
}
func (p *pool) Stop() error {
	return p.parent.Stop()
}

type item struct {
	parent cpool.StatusItem
	port   string
}

func NewItem(parent cpool.StatusItem) Item {
	ports := cpool.TCPPortsFor(parent.Status())
	return &item{parent, ports[strconv.Itoa(defaultPort)]}
}

func (i *item) ID() string {
	return i.parent.ID()
}

func (i *item) Host() string {
	return "localhost"
}

func (i *item) Port() string {
	return i.port
}

func (i *item) Database() string {
	return "0"
}

func (i *item) URL() string {
	return fmt.Sprintf("redis://%v:%v/%v",
		url.QueryEscape(i.Host()), url.QueryEscape(i.Port()), url.QueryEscape(i.Database()))
}

func LiveCheck(timeout time.Duration, tries int, delay time.Duration, fn func(context.Context, Item) error) cpool.ProvisionFn {
	return cpool.LiveCheck(timeout, tries, delay, func(ctx context.Context, si cpool.StatusItem) error {
		return fn(ctx, NewItem(si))
	})
}

func RediGoLiveCheck() func(context.Context, Item) error {
	return func(ctx context.Context, item Item) error {

		fmt.Printf("url: %v\n", item.URL())

		conn, err := rredis.DialURL(item.URL())

		if err != nil {
			return err
		}
		_, err = conn.Do("PING")
		return err
	}
}
