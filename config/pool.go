package config

import "github.com/boz/ephemerald/lifecycle"

type Pool struct {
	Name      string
	Size      int
	Image     string
	Port      int
	Container *Container
	Actions   lifecycle.Config
}
