package config

import (
	"github.com/boz/ephemerald/lifecycle"
	"github.com/boz/ephemerald/params"
)

type Pool struct {
	Name      string
	Size      int
	Image     string
	Port      int
	Container *Container
	Params    params.Config
	Actions   lifecycle.Config
}
