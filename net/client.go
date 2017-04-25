package net

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"

	"github.com/boz/ephemerald/params"
)

var (
	DefaultConnectAddress = net.JoinHostPort("localhost", strconv.Itoa(DefaultPort))
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

func (c *Client) Checkout(names ...string) (params.Set, error) {
	ps := params.Set{}

	req, err := http.NewRequest("PUT", c.url(rpcCheckoutName), &bytes.Buffer{})

	if err != nil {
		return ps, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ps, err
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ps, err
	}

	err = json.Unmarshal(buf, &ps)
	return ps, err
}

func (c *Client) Return(ps params.Set) error {
	buf, err := json.Marshal(ps)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", c.url(rpcReturnName), bytes.NewBuffer(buf))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) url(path string) string {
	return fmt.Sprintf("http://%v%v", c.address, path)
}
