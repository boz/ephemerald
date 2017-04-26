package net

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/boz/ephemerald"
	"github.com/boz/ephemerald/params"
)

type ClientBuilder struct {
	address string
}

type Client struct {
	address string
}

func NewClientBuilder() *ClientBuilder {
	return &ClientBuilder{DefaultConnectAddress}
}

func (b *ClientBuilder) WithAddress(address string) *ClientBuilder {
	b.address = address
	return b
}

func (b *ClientBuilder) WithPort(port int) *ClientBuilder {
	address, _, _ := net.SplitHostPort(b.address)
	b.address = net.JoinHostPort(address, strconv.Itoa(port))
	return b
}

func (b *ClientBuilder) Create() (*Client, error) {
	return &Client{b.address}, nil
}

func (c *Client) CheckoutBatch(names ...string) (params.Set, error) {
	ps := params.Set{}

	req, err := http.NewRequest("POST", c.url(rpcCheckoutPath), &bytes.Buffer{})
	if err != nil {
		return ps, err
	}
	req.Header.Add("Content-Type", rpcContentType)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ps, err
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&ps)
	return ps, err
}

func (c *Client) Checkout(name string) (params.Params, error) {
	params := params.Params{}

	req, err := http.NewRequest("POST", c.url(rpcCheckoutPath, name), &bytes.Buffer{})
	if err != nil {
		return params, err
	}
	req.Header.Add("Content-Type", rpcContentType)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return params, err
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&params)
	return params, err
}

func (c *Client) ReturnBatch(ps params.Set) error {
	buf, err := json.Marshal(ps)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("DELETE", c.url(rpcReturnPath), bytes.NewBuffer(buf))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", rpcContentType)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) Return(name string, item ephemerald.Item) error {

	url := c.url(rpcReturnPath, name, item.ID())

	req, err := http.NewRequest("DELETE", url, &bytes.Buffer{})
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", rpcContentType)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) url(path string, parts ...string) string {
	for _, part := range parts {
		path = path + "/" + url.QueryEscape(part)
	}
	return fmt.Sprintf("http://%v%v", c.address, path)
}
