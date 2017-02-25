package pg

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"time"

	_ "github.com/lib/pq"

	"github.com/ovrclk/cleanroom"
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
	Create() (Pool, error)
}

type ProvisionFn func(context.Context, *Item) error

func MakeProvisioner(fn ProvisionFn) cleanroom.ProvisionFn {
	return func(ctx context.Context, si cleanroom.StatusItem) error {
		return fn(ctx, NewItem(si))
	}
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

func NewItem(parent cleanroom.StatusItem) *Item {
	port := cleanroom.TCPPortFor(parent.Status(), defaultPort)
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

func LiveCheck(timeout time.Duration, tries int, delay time.Duration, fn ProvisionFn) cleanroom.ProvisionFn {
	return cleanroom.LiveCheck(timeout, tries, delay, MakeProvisioner(fn))
}

func PQPingLiveCheck() ProvisionFn {
	return func(ctx context.Context, item *Item) error {
		db, err := sql.Open("postgres", item.URL)
		if err != nil {
			fmt.Printf("error:%v\n", err)
			return err
		}
		return db.Ping()
	}
}
