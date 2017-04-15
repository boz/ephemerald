package net

import (
	"sync"

	"github.com/koding/kite"
	"github.com/ovrclk/cleanroom/builtin/pg"
	"github.com/ovrclk/cleanroom/builtin/redis"
)

const (
	kiteName    = "cleanroom"
	kiteVersion = "0.0.1"
	kitePort    = 6000
)

type PoolServer interface {
	WaitReady() error
	Stop() error
}

type Server struct {
	kite *kite.Kite

	redis PoolServer
	pg    PoolServer
	vault PoolServer

	closech chan bool
}

func NewServer() (*Server, error) {
	return NewServerWithPort(kitePort)
}

func NewServerWithPort(port int) (*Server, error) {
	k := kite.New(kiteName, kiteVersion)

	k.Config.Port = port
	k.Config.DisableAuthentication = true

	redis, err := redis.BuildServer(k)
	if err != nil {
		return nil, err
	}

	pg, err := pg.BuildServer(k)
	if err != nil {
		redis.Stop()
		return nil, err
	}

	return &Server{
		kite:    k,
		redis:   redis,
		pg:      pg,
		closech: make(chan bool),
	}, nil
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
	var wg sync.WaitGroup
	wg.Add(2)

	fn := func(p PoolServer) {
		defer wg.Done()
		p.Stop()
	}

	go fn(s.redis)
	go fn(s.pg)

	wg.Wait()
}
