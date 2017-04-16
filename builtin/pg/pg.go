package pg

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"time"

	// needed for livecheck
	_ "github.com/lib/pq"

	"github.com/boz/ephemerald"
)

const (
	defaultImage = "postgres"
	defaultPort  = 5432

	defaultHost     = "localhost"
	defaultUser     = "postgres"
	defaultDatabase = "postgres"
	defaultPassword = ""
)

type Item struct {
	Cid string

	Host     string
	Port     string
	User     string
	Database string
	Password string

	URL string
}

type Pool interface {
	Checkout() (*Item, error)
	Return(*Item)
	Stop() error
	WaitReady() error
}

type Builder interface {
	WithDefaults() Builder
	WithImage(string) Builder
	WithSize(int) Builder
	WithLiveCheck(ProvisionFn) Builder
	WithInitialize(ProvisionFn) Builder
	WithReset(ProvisionFn) Builder
	WithLabel(string, string) Builder
	Clone() Builder
	Create() (Pool, error)
}

type ProvisionFn func(context.Context, *Item) error

func MakeProvisioner(fn ProvisionFn) ephemerald.ProvisionFn {
	return func(ctx context.Context, si ephemerald.StatusItem) error {
		return fn(ctx, NewItem(si))
	}
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
			PQPingLiveCheck()))

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

func (b *builder) WithLabel(k, v string) Builder {
	b.config.WithLabel(k, v)
	return b
}

func (b *builder) WithLiveCheck(fn ProvisionFn) Builder {
	b.pbuilder.WithLiveCheck(MakeProvisioner(fn))
	return b
}

func (b *builder) WithInitialize(fn ProvisionFn) Builder {
	b.pbuilder.WithInitialize(MakeProvisioner(fn))
	return b
}

func (b *builder) WithReset(fn ProvisionFn) Builder {
	b.pbuilder.WithReset(MakeProvisioner(fn))
	return b
}

func (b *builder) Clone() Builder {
	return &builder{
		size:     b.size,
		config:   &(*b.config),
		pbuilder: b.pbuilder.Clone(),
	}
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

func NewItem(parent ephemerald.StatusItem) *Item {
	port := ephemerald.TCPPortFor(parent.Status(), defaultPort)
	item := &Item{
		Cid:      parent.ID(),
		Host:     defaultHost,
		Port:     port,
		User:     defaultUser,
		Database: defaultDatabase,
		Password: defaultPassword,
	}
	item.URL = genURL(item)
	return item
}

func (i *Item) ID() string {
	return i.Cid
}

func genURL(item *Item) string {
	ui := url.UserPassword(item.User, item.Password)

	return fmt.Sprintf("postgres://%v@%v:%v/%v?sslmode=disable",
		ui.String(),
		url.QueryEscape(item.Host),
		url.QueryEscape(item.Port),
		url.QueryEscape(item.Database))
}

func LiveCheck(timeout time.Duration, tries int, delay time.Duration, fn ProvisionFn) ephemerald.ProvisionFn {
	return ephemerald.LiveCheck(timeout, tries, delay, MakeProvisioner(fn))
}

func PQPingLiveCheck() ProvisionFn {
	return func(ctx context.Context, item *Item) error {
		db, err := sql.Open("postgres", item.URL)
		if err != nil {
			return err
		}
		defer db.Close()
		return db.Ping()
	}
}
