package redis

import "github.com/koding/kite"

const (
	rpcCheckoutName = "redis.checkout"
	rpcReturnName   = "redis.return"
)

type Server struct {
	pool Pool
	kite *kite.Kite
}

func BuildServer(kite *kite.Kite) (*Server, error) {
	pool, err := DefaultBuilder().WithSize(10).Create()

	if err != nil {
		return nil, err
	}

	s := &Server{pool, kite}

	kite.HandleFunc(rpcCheckoutName, s.handleCheckout)
	kite.HandleFunc(rpcReturnName, s.handleReturn)

	return s, nil
}

func (s *Server) WaitReady() error {
	return s.pool.WaitReady()
}

func (s *Server) Stop() error {
	return s.pool.Stop()
}

func (s *Server) handleCheckout(r *kite.Request) (interface{}, error) {
	s.kite.Log.Info(">redis.checkout")
	item, err := s.pool.Checkout()
	if err != nil {
		return nil, err
	}
	s.kite.Log.Info("<redis.checkout")
	return s.transformItem(r, item), nil
}

func (s *Server) handleReturn(r *kite.Request) (interface{}, error) {
	s.kite.Log.Info(">redis.return")
	item := Item{}
	r.Args.One().MustUnmarshal(&item)
	s.pool.Return(&item)
	s.kite.Log.Info("<redis.return")
	return nil, nil
}

func (s *Server) transformItem(r *kite.Request, item *Item) *Item {
	item.Host = r.Client.Environment
	item.URL = makeItemURL(item)
	return item
}
