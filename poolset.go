package ephemerald

import (
	"context"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/params"
)

type PoolSet interface {
	Checkout(name ...string) (params.Set, error)
	CheckoutWith(ctx context.Context, name ...string) (params.Set, error)
	ReturnAll(params.Set)
	Return(name string, item Item)
	WaitReady() error
	Stop() error
}

type poolSet struct {
	pools map[string]Pool
	log   logrus.FieldLogger
}

func NewPoolSet(log logrus.FieldLogger, ctx context.Context, configs []*config.Config) (PoolSet, error) {
	pools := make(map[string]Pool)

	log = log.WithField("component", "pool-set")

	var err error
	var pool Pool
	for _, cfg := range configs {
		pool, err = NewPoolWithContext(ctx, cfg)
		if err != nil {
			break
		}
		pools[cfg.Name] = pool
	}

	if err != nil {
		var wg sync.WaitGroup
		wg.Add(len(pools))
		for _, pool := range pools {
			go func(pool Pool) {
				defer wg.Done()
				pool.Stop()
			}(pool)
		}
		wg.Wait()
		return nil, err
	}

	return &poolSet{
		pools: pools,
		log:   log,
	}, nil
}

func (ps *poolSet) CheckoutWith(ctx context.Context, names ...string) (params.Set, error) {
	type pentry struct {
		name string
		p    params.Params
		err  error
	}

	ch := make(chan pentry)

	var wg sync.WaitGroup

	for name, pool := range ps.pools {
		if len(names) > 0 {
			found := false
			for _, n := range names {
				if name == n {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		wg.Add(1)
		go func(name string, pool Pool) {
			defer wg.Done()
			p, err := pool.CheckoutWith(ctx)
			ch <- pentry{name, p, err}
		}(name, pool)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	set := params.Set{}

	var cerr error
	for entry := range ch {
		if entry.err != nil {
			cerr = entry.err
			continue
		}
		set[entry.name] = entry.p
	}

	if cerr != nil {
		var wg sync.WaitGroup
		wg.Add(len(set))
		for name, p := range set {
			go func(name string, p params.Params) {
				defer wg.Done()
				ps.pools[name].Return(p)
			}(name, p)
		}
		wg.Wait()
		return nil, cerr
	}
	return set, nil
}

func (ps *poolSet) Checkout(names ...string) (params.Set, error) {
	return ps.CheckoutWith(context.Background(), names...)
}

func (ps *poolSet) ReturnAll(set params.Set) {
	var wg sync.WaitGroup
	wg.Add(len(set))

	for name, p := range set {
		go func(name string, p params.Params) {
			defer wg.Done()
			ps.Return(name, p)
		}(name, p)
	}

	wg.Wait()
}

func (ps *poolSet) Return(name string, item Item) {
	if pool, ok := ps.pools[name]; ok {
		pool.Return(item)
	}
}

func (ps *poolSet) WaitReady() error {
	ch := make(chan error)
	defer close(ch)

	for _, pool := range ps.pools {
		go func(pool Pool) {
			ch <- pool.WaitReady()
		}(pool)
	}

	var err error
	for count := 0; count < len(ps.pools); count++ {
		if perr := <-ch; perr != nil {
			if err == nil {
				err = perr
			}
		}
	}
	return err
}

func (ps *poolSet) Stop() error {
	var wg sync.WaitGroup
	wg.Add(len(ps.pools))

	for name, pool := range ps.pools {
		go func(name string, pool Pool) {
			defer wg.Done()
			pool.Stop()
		}(name, pool)
	}

	wg.Wait()
	return nil
}
