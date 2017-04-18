package config

import (
	"fmt"
)

func NewConfig(name string) *Config {
	return &Config{
		Name:      name,
		Container: NewContainer(),
	}
}

func (c *Config) WithImage(image string) *Config {
	c.Image = image
	return c
}

func (c *Config) WithEnv(name, value string) *Config {
	c.Container.Env = append(c.Container.Env, fmt.Sprintf("%v=%v", name, value))
	return c
}

func (c *Config) WithLabel(k, v string) *Config {
	c.Container.Labels[k] = v
	return c
}
