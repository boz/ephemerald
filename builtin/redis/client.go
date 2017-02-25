package redis

import "github.com/koding/kite"

type Client struct {
	kclient *kite.Client
}

func BuildClient(k *kite.Client) *Client {
	return &Client{k}
}

func (c *Client) Checkout() (*Item, error) {
	response, err := c.kclient.Tell(rpcCheckoutName)
	if err != nil {
		return nil, err
	}
	i := Item{}
	response.One().MustUnmarshal(&i)
	return &i, nil
}

func (c *Client) Return(i *Item) error {
	_, err := c.kclient.Tell(rpcReturnName, i)
	return err
}
