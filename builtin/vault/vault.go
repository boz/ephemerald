package vault

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ovrclk/cleanroom"
)

const (
	defaultImage = "vault"
	defaultPort  = 8200
)

type Item interface {
	ID() string

	Host() string
	Port() string
	URL() string
}

type Pool interface {
	Checkout() (Item, error)
	Return(Item)
	WaitReady() error
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
	return NewBuilder().WithDefaults()
}

func (b *builder) WithDefaults() Builder {

	applyDefaults(b.config)

	b.pbuilder.WithLiveCheck(
		LiveCheck(
			cleanroom.LiveCheckDefaultTimeout,
			cleanroom.LiveCheckDefaultRetries,
			cleanroom.LiveCheckDefaultDelay,
			HTTPLiveCheck()))

	return b
}

func applyDefaults(config *cleanroom.Config) *cleanroom.Config {
	return config.
		WithImage(defaultImage).
		ExposePort("tcp", defaultPort).
		WithEnv("SKIP_SETCAP", "1")
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

func (i *item) URL() string {
	return fmt.Sprintf("http://%v:%v",
		url.QueryEscape(i.Host()), url.QueryEscape(i.Port()))
}

func LiveCheck(timeout time.Duration, tries int, delay time.Duration, fn func(context.Context, Item) error) cleanroom.ProvisionFn {
	return cleanroom.LiveCheck(timeout, tries, delay, func(ctx context.Context, si cleanroom.StatusItem) error {
		return fn(ctx, NewItem(si))
	})
}

func HTTPLiveCheck() func(context.Context, Item) error {
	return func(ctx context.Context, item Item) error {
		return Ping(ctx, item.URL())
	}
}

func Ping(ctx context.Context, url string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req = req.WithContext(ctx)

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(ioutil.Discard, resp.Body)

	return err
}
