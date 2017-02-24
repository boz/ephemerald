package redis

import "github.com/koding/kite"

type NetPool struct {
	pool Pool
}

func Build() *NetPool {
	pool := Builder().WithDefaults().Create()
	return &NetPool{pool}
}

func (p *pool) HandleCheckout(r *kite.Request) (interface{}, error) {
	item, err := p.pool.Checkout()
	if err != nil {
		return netItem(item), nil
	}
	return nil, err
}

func (p *pool) HandleReturn(r *kite.Request) (interface{}, error) {
	item := Item{}
	if err := r.Args.One().Unmarshal(&item); err != nil {
		return nil, error
	}
	p.pool.Return(&item)
	return nil, nil
}
