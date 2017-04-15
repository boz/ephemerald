package ephemerald

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-connections/nat"
)

type Config struct {
	Image    string
	Cmd      strslice.StrSlice
	Ports    nat.PortMap
	Env      []string
	Volumes  map[string]struct{}
	LogLevel logrus.Level
	Labels   map[string]string
}

func NewConfig() *Config {
	return &Config{
		Ports:    make(nat.PortMap),
		Volumes:  make(map[string]struct{}),
		LogLevel: logrus.DebugLevel,
		Labels:   make(map[string]string),
	}
}

func (c *Config) WithImage(image string) *Config {
	c.Image = image
	return c
}

func (c *Config) WithEnv(name, value string) *Config {
	c.Env = append(c.Env, fmt.Sprintf("%v=%v", name, value))
	return c
}

func (c *Config) WithLogLevel(level logrus.Level) *Config {
	c.LogLevel = level
	return c
}

func (c *Config) WithLabel(k, v string) *Config {
	c.Labels[k] = v
	return c
}

func (c *Config) ExposePort(net string, port int) *Config {
	return c
}

func (c *Config) toHostConfig() *container.HostConfig {
	return &container.HostConfig{
		AutoRemove: true,
	}
}
