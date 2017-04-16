package net

import (
	"sync"

	"github.com/boz/ephemerald/builtin/pg"
	"github.com/boz/ephemerald/builtin/redis"
	"github.com/koding/kite"
)

const (
	kiteName    = "ephemerald"
	kiteVersion = "0.0.1"
	DefaultPort = 6000
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

type ServerBuilder struct {
	port int

	pgBuilder    pg.Builder
	redisBuilder redis.Builder
}

func NewServerBuilder() *ServerBuilder {
	return &ServerBuilder{
		port: DefaultPort,
	}
}

func (sb *ServerBuilder) WithPort(port int) *ServerBuilder {
	sb.port = port
	return sb
}

func (sb *ServerBuilder) PG() pg.Builder {
	if sb.pgBuilder == nil {
		sb.pgBuilder = pg.NewBuilder().WithDefaults()
	}
	return sb.pgBuilder
}

func (sb *ServerBuilder) Redis() redis.Builder {
	if sb.redisBuilder == nil {
		sb.redisBuilder = redis.NewBuilder().WithDefaults()
	}
	return sb.redisBuilder
}

func (sb *ServerBuilder) Create() (*Server, error) {
	k := kite.New(kiteName, kiteVersion)

	k.Config.Port = sb.port
	k.Config.DisableAuthentication = true

	s := &Server{
		kite:    k,
		closech: make(chan bool),
	}

	if sb.pgBuilder != nil {
		server, err := pg.BuildServer(k, sb.pgBuilder)
		if err != nil {
			return nil, err
		}
		s.pg = server
	}

	if sb.redisBuilder != nil {
		server, err := redis.BuildServer(k, sb.redisBuilder)
		if err != nil {
			s.pg.Stop()
			return nil, err
		}
		s.redis = server
	}

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
	var wg sync.WaitGroup

	fn := func(p PoolServer) {
		defer wg.Done()
		p.Stop()
	}

	if s.redis != nil {
		wg.Add(1)
		go fn(s.redis)
	}

	if s.pg != nil {
		wg.Add(1)
		go fn(s.pg)
	}

	wg.Wait()
}
