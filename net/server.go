package net

import (
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
}

func NewServer() (*Server, error) {
	k := kite.New(kiteName, kiteVersion)

	k.SetLogLevel(kite.DEBUG)

	k.Config.Port = kitePort
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
		kite:  k,
		redis: redis,
		pg:    pg,
	}, nil
}
