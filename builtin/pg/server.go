package pg

import (
	"context"
	"fmt"
	"sync"

	"github.com/koding/kite"
)

type Server struct {
	kite  *kite.Kite
	pools map[string]Pool

	mtx sync.Mutex
}

func BuildServer(k *kite.Kite) (*Server, error) {

	s := &Server{k, make(map[string]Pool), sync.Mutex{}}

	k.HandleFunc(rpcCheckoutName, s.handleCheckout)
	k.HandleFunc(rpcReturnName, s.handleReturn)

	k.OnFirstRequest(s.startSession)
	k.OnDisconnect(s.stopSession)

	return s, nil
}

func (s *Server) WaitReady() error {
	return nil
}

func (s *Server) Stop() error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	var wg sync.WaitGroup
	wg.Add(len(s.pools))
	for _, pool := range s.pools {
		go func() {
			defer wg.Done()
			pool.Stop()
		}()
	}
	s.pools = make(map[string]Pool)
	wg.Wait()
	return nil
}

func (s *Server) startSession(c *kite.Client) {
	pool, err := DefaultBuilder().
		WithSize(2).
		WithInitialize(func(ctx context.Context, i *Item) error {
			return RemoteDo(ctx, c, rpcInitializeName, i)
		}).
		WithReset(func(ctx context.Context, i *Item) error {
			return RemoteDo(ctx, c, rpcResetName, i)
		}).Create()

	if err != nil {
		s.kite.Log.Error("pg: can't initialize session %v: %v", c.ID, err)
		c.LocalKite.Log.Error("error: can't create postgres pool: %v", err)
		return
	}

	if err := pool.WaitReady(); err != nil {
		s.kite.Log.Error("pg: error waiting for pool %v: %v", err)
		c.LocalKite.Log.Error("error: unable to create postgres pool: %v", err)
		pool.Stop()
		return
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.pools[c.ID] = pool
}

func (s *Server) stopSession(c *kite.Client) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if pool, ok := s.pools[c.ID]; ok {
		go func() {
			if err := pool.Stop(); err != nil {
				s.kite.Log.Error("error stopping pool for %v: %v", err)
			}
		}()
		delete(s.pools, c.ID)
		return
	}
	s.kite.Log.Warning("stopping unknown session: %v", c.ID)
}

func (s *Server) handleCheckout(r *kite.Request) (interface{}, error) {
	s.kite.Log.Info(">pg.checkout")
	pool, err := s.poolForSession(r)
	if err != nil {
		return nil, err
	}
	i, err := pool.Checkout()
	if err != nil {
		return nil, err
	}
	s.kite.Log.Info("<pg.checkout")
	return transformItem(r.Client, i), nil
}

func (s *Server) handleReturn(r *kite.Request) (interface{}, error) {
	s.kite.Log.Info(">pg.return")
	i := Item{}
	r.Args.One().MustUnmarshal(&i)

	pool, err := s.poolForSession(r)
	if err != nil {
		return nil, err
	}

	pool.Return(&i)

	s.kite.Log.Info("<pg.return")
	return nil, nil
}

func (s *Server) poolForSession(r *kite.Request) (Pool, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	pool, ok := s.pools[r.Client.ID]
	if !ok {
		return nil, fmt.Errorf("no pool found for session %v", r.Client.ID)
	}
	return pool, nil
}

func RemoteDo(ctx context.Context, c *kite.Client, name string, i *Item) error {
	ch := c.Go(name, transformItem(c, i))
	select {
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-ch:
		return resp.Err
	}
}

func transformItem(c *kite.Client, i *Item) *Item {
	i.Host = c.Environment
	i.URL = genURL(i)
	return i
}
