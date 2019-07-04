package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"path"

	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/log"
	"github.com/boz/ephemerald/types"
	"github.com/sirupsen/logrus"
)

const (
	defaultAddress = "localhost:6000"
	contentType    = "application/json"
	poolBasePath   = "/pool"
)

type Interface interface {
	Pool() PoolInterface
}

type PoolInterface interface {
	Create(context.Context, config.Pool) (*types.Pool, error)
	Get(context.Context, types.ID) (*types.Pool, error)
	List(context.Context) ([]types.Pool, error)
	Delete(context.Context, types.ID) error

	Checkout(context.Context, types.ID) (*types.Checkout, error)
	Release(context.Context, types.ID, types.ID) error
}

type Opt func(*client) error

func WithHost(host string) Opt {
	return func(c *client) error {
		c.host = host
		return nil
	}
}

func WithLog(l logrus.FieldLogger) Opt {
	return func(c *client) error {
		c.l = l
		return nil
	}
}

type client struct {
	host  string
	chttp *http.Client
	l     logrus.FieldLogger
}

func New(opts ...Opt) (Interface, error) {
	c := &client{
		host:  defaultAddress,
		l:     log.Default(),
		chttp: &http.Client{},
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (c *client) Pool() PoolInterface {
	return c
}

func (c *client) Create(ctx context.Context, cfg config.Pool) (*types.Pool, error) {
	buf, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(ctx, "POST", poolBasePath, bytes.NewBuffer(buf))
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}

	buf, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var pool types.Pool
	if err := json.Unmarshal(buf, &pool); err != nil {
		return nil, err
	}
	return &pool, nil
}

func (c *client) Get(ctx context.Context, id types.ID) (*types.Pool, error) {
	resp, err := c.doRequest(ctx, "GET", path.Join(poolBasePath, string(id)), nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var pool types.Pool
	if err := json.Unmarshal(buf, &pool); err != nil {
		return nil, err
	}

	return &pool, nil
}

func (c *client) List(ctx context.Context) ([]types.Pool, error) {
	resp, err := c.doRequest(ctx, "GET", poolBasePath+"s", nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var pools []types.Pool
	if err := json.Unmarshal(buf, &pools); err != nil {
		return nil, err
	}
	return pools, nil
}

func (c *client) Delete(ctx context.Context, id types.ID) error {
	resp, err := c.doRequest(ctx, "DELETE", path.Join(poolBasePath, string(id)), nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	return err
}

func (c *client) Checkout(ctx context.Context, id types.ID) (*types.Checkout, error) {
	resp, err := c.doRequest(ctx, "POST", path.Join(poolBasePath, string(id), "checkout"), nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var obj types.Checkout
	if err := json.Unmarshal(buf, &obj); err != nil {
		return nil, err
	}
	return &obj, nil
}

func (c *client) Release(ctx context.Context, pid types.ID, id types.ID) error {
	resp, err := c.doRequest(ctx, "DELETE", path.Join(poolBasePath, string(pid), "checkout", string(id)), nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	return err
}

func (c *client) doRequest(ctx context.Context, method string, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, c.host+path, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Add("Content-Type", contentType)

	return c.chttp.Do(req)
}
