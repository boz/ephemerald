package config

import "github.com/docker/docker/api/types/strslice"

type Container struct {
	// docker/docker/api/types/container/config.go
	Labels map[string]string

	// unused
	Env        []string
	Cmd        strslice.StrSlice
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
