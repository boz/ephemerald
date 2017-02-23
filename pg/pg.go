package pg

import (
	"context"

	"github.com/ovrclk/cpool"
)

type Item interface {
	ID() string

	Host() string
	Port() string
	User() string
	Database() string
	Password() string
}

type Pool interface {
	Checkout() Item
	Return(Item)
	Stop() error
}

type Builder interface {
	WithImage(string) Builder
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
		WithImage("postgres").
		ExposePort("tcp", 5432)
}

func NewBuilder() Builder {
	return &builder{1, DefaultConfig(), cpool.BuildProvisioner()}
}

func (b *builder) WithSize(size int) Builder {
	b.size = size
	return b
}

func (b *builder) WithImage(name string) Builder {
	b.config.WithImage(name)
	return b
}

func (b *builder) WithInitialize(fn func(context.Context, Item) error) Builder {
	b.pbuilder.WithInitialize(func(ctx context.Context, item cpool.StatusItem) error {
		return fn(ctx, NewItem(item))
	})
	return b
}

func (b *builder) WithReset(fn func(context.Context, Item) error) Builder {
	b.pbuilder.WithReset(func(ctx context.Context, item cpool.StatusItem) error {
		return fn(ctx, NewItem(item))
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
	return &item{parent, ports["5432"]}
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
