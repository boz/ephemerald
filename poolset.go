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

// ugh.

type poolSet struct {
	pools map[string]Pool
	log   logrus.FieldLogger
}

func NewPoolSet(log logrus.FieldLogger, ctx context.Context, configs []*config.Config) (PoolSet, error) {
	pools := make(map[string]Pool)

	log = log.WithField("component", "pool-set")

	var err error
	var pool Pool

	// create pools
	for _, cfg := range configs {
		pool, err = NewPoolWithContext(ctx, cfg)
		if err != nil {
			break
		}
		pools[cfg.Name] = pool
	}

	if err == nil {
		return &poolSet{
			pools: pools,
			log:   log,
		}, nil
	}

	// if creation failed for any pool, stop active pools
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

func (ps *poolSet) CheckoutWith(ctx context.Context, names ...string) (params.Set, error) {
	type pscheckout struct {
		name   string
		params params.Params
		err    error
	}

	// checkout from each pool
	ch := make(chan pscheckout)
	var wg sync.WaitGroup
	for name, pool := range ps.poolsForCheckout(names...) {
		wg.Add(1)
		go func(name string, pool Pool) {
			defer wg.Done()
			params, err := pool.CheckoutWith(ctx)
			ch <- pscheckout{name, params, err}
		}(name, pool)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	// collect results
	set := params.Set{}
	errors := make([]error, 0)
	for entry := range ch {
		if entry.err != nil {
			ps.log.WithError(entry.err).
				WithField("pool", entry.name).
				Error("checkout error")
			errors = append(errors, entry.err)
			continue
		}
		set[entry.name] = entry.params
	}

	// if any checkout failed, return completed checkouts
	if len(errors) > 0 {
		ps.ReturnAll(set)
		return nil, errors[0]
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
	type pswait struct {
		name string
		pool Pool
		err  error
	}

	ch := make(chan pswait)

	// call WaitReady on all pools
	for name, pool := range ps.pools {
		go func(name string, pool Pool) {
			ch <- pswait{name, pool, pool.WaitReady()}
		}(name, pool)
	}

	// collect results
	var err error
	for count := 0; count < len(ps.pools); count++ {
		if response := <-ch; response.err != nil {

			ps.log.WithError(response.err).
				WithField("pool", response.name).
				Error("WaitReady failed")

			if err == nil {
				err = response.err
			}
		}
	}
	return err
}

func (ps *poolSet) Stop() error {
	var wg sync.WaitGroup
	wg.Add(len(ps.pools))

	// stop all pools
	for name, pool := range ps.pools {
		go func(name string, pool Pool) {
			defer wg.Done()
			pool.Stop()
		}(name, pool)
	}

	// wait for all pools to stop
	wg.Wait()
	return nil
}

func (ps *poolSet) poolsForCheckout(names ...string) map[string]Pool {
	// if no names given, all pools returned
	if len(names) == 0 {
		return ps.pools
	}

	// else select the pools by name
	pools := make(map[string]Pool)
	for _, name := range names {
		if pool, ok := ps.pools[name]; ok {
			pools[name] = pool
		}
	}
	return pools
}
