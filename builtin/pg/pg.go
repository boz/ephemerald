package pg

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/ovrclk/cleanroom"
)

const (
	defaultImage = "postgres"
	defaultPort  = 5432
)

type Item interface {
	ID() string

	Host() string
	Port() string
	User() string
	Database() string
	Password() string

	URL() string
}

type Pool interface {
	Checkout() (Item, error)
	Return(Item)
	Stop() error
	WaitReady() error
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
			PQPingLiveCheck()))

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

func (b *builder) WithLiveCheck(fn func(context.Context, Item) error) Builder {
	b.pbuilder.WithLiveCheck(func(ctx context.Context, si cleanroom.StatusItem) error {
		return fn(ctx, NewItem(si))
	})
	return b
}

func (b *builder) WithInitialize(fn func(context.Context, Item) error) Builder {
	b.pbuilder.WithInitialize(func(ctx context.Context, si cleanroom.StatusItem) error {
		return fn(ctx, NewItem(si))
	})
	return b
}

func (b *builder) WithReset(fn func(context.Context, Item) error) Builder {
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

func (p *pool) Checkout() (Item, error) {
	item, err := p.parent.Checkout()
	if err != nil {
		return nil, err
	}
	return NewItem(item), nil
}
func (p *pool) Return(item Item) {
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

func NewItem(parent cleanroom.StatusItem) Item {
	ports := cleanroom.TCPPortsFor(parent.Status())
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

func (i *item) User() string {
	return "postgres"
}

func (i *item) Database() string {
	return "postgres"
}

func (i *item) Password() string {
	return ""
}

func (i *item) URL() string {
	ui := url.UserPassword(i.User(), i.Password())
	return fmt.Sprintf("postgres://%v@%v:%v/%v?sslmode=disable",
		ui.String(), url.QueryEscape(i.Host()), url.QueryEscape(i.Port()), url.QueryEscape(i.Database()))
}

func LiveCheck(timeout time.Duration, tries int, delay time.Duration, fn func(context.Context, Item) error) cleanroom.ProvisionFn {
	return cleanroom.LiveCheck(timeout, tries, delay, func(ctx context.Context, si cleanroom.StatusItem) error {
		return fn(ctx, NewItem(si))
	})
}

func PQPingLiveCheck() func(context.Context, Item) error {
	return func(ctx context.Context, item Item) error {
		db, err := sql.Open("postgres", item.URL())
		if err != nil {
			return err
		}
		return db.Ping()
	}
}
