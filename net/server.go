package net

import (
	"github.com/boz/ephemerald"
	"github.com/boz/ephemerald/params"
	"github.com/koding/kite"
)

const (
	kiteName    = "ephemerald"
	kiteVersion = "0.0.1"
	DefaultPort = 6000
)

type Server struct {
	kite *kite.Kite

	pools ephemerald.PoolSet

	closech chan bool
}

type ServerBuilder struct {
	port  int
	pools ephemerald.PoolSet
}

func NewServerBuilder() *ServerBuilder {
	return &ServerBuilder{
		port: DefaultPort,
	}
}

func (sb *ServerBuilder) WithPoolSet(pools ephemerald.PoolSet) *ServerBuilder {
	sb.pools = pools
	return sb
}

func (sb *ServerBuilder) WithPort(port int) *ServerBuilder {
	sb.port = port
	return sb
}

func (sb *ServerBuilder) Create() (*Server, error) {
	k := kite.New(kiteName, kiteVersion)

	k.Config.Port = sb.port
	k.Config.DisableAuthentication = true

	s := &Server{
		kite:    k,
		closech: make(chan bool),
		pools:   sb.pools,
	}

	s.kite.HandleFunc(rpcCheckoutName, s.handleCheckout)
	s.kite.HandleFunc(rpcReturnName, s.handleReturn)

	return s, nil
}

func (s *Server) Run() {
	defer close(s.closech)
	ch := s.kite.ServerCloseNotify()
	go s.kite.Run()
	<-ch
	s.stopPools()
}

func (s *Server) Close() {
	s.kite.Close()
}

func (s *Server) ServerCloseNotify() chan bool {
	return s.closech
}

func (s *Server) ServerReadyNotify() chan bool {
	return s.kite.ServerReadyNotify()
}

func (s *Server) Port() int {
	return s.kite.Port()
}

func (s *Server) stopPools() {
	s.pools.Stop()
}

func (s *Server) handleCheckout(r *kite.Request) (interface{}, error) {
	var names []string
	r.Args.MustUnmarshal(names)
	ps, err := s.pools.Checkout(names...)
	if err != nil {
		return ps, err
	}

	host := r.Client.Environment

	for name, p := range ps {
		p2, e := p.ForHost(host)
		if e != nil {
			err = e
			break
		}
		ps[name] = p2
	}
	if err != nil {
		s.pools.ReturnAll(ps)
	}
	return ps, err
}

func (s *Server) handleReturn(r *kite.Request) (interface{}, error) {
	ps := params.ParamSet{}
	r.Args.One().MustUnmarshal(&ps)
	s.pools.ReturnAll(ps)
	return nil, nil
}
