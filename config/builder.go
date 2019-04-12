package config

import (
	"fmt"
)

func NewPool(name string) *Pool {
	return &Pool{
		Name:      name,
		Container: NewContainer(),
	}
}

func (c *Pool) WithImage(image string) *Pool {
	c.Image = image
	return c
}

func (c *Pool) WithEnv(name, value string) *Pool {
	c.Container.Env = append(c.Container.Env, fmt.Sprintf("%v=%v", name, value))
	return c
}

func (c *Pool) WithLabel(k, v string) *Pool {
	c.Container.Labels[k] = v
	return c
}
