package config

import (
	"github.com/boz/ephemerald/lifecycle"
	"github.com/docker/docker/api/types/strslice"
)

type Pool struct {
	Name      string            `json:"name"`
	Size      int               `json:"size"`
	Image     string            `json:"image"`
	Port      int               `json:"port"`
	Container *Container        `json:"container,omitempty"`
	Params    map[string]string `json:"params,omitempty"`
	Actions   lifecycle.Config  `json:"actions,omitempty"`
}

type Container struct {
	// docker/docker/api/types/container/config.go
	Labels map[string]string `json:"labels,omitempty"`

	// unused
	Env        []string          `json:"env,omitempty"`
	Cmd        strslice.StrSlice `json:"cmd,omitempty"`
	Volumes    map[string]struct{}
	Entrypoint strslice.StrSlice // Entrypoint to run when starting the container
	User       string

	// docker/docker/api/types/container/host_config.go
	CapAdd  strslice.StrSlice
	CapDrop strslice.StrSlice
}

func NewContainer() *Container {
	return &Container{
		Labels:  make(map[string]string),
		Volumes: make(map[string]struct{}),
	}
}
