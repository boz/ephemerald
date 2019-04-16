package lifecycle

import (
	"context"
	"fmt"

	"github.com/boz/ephemerald/params"
)

var (
	ErrActionNotConfigured = fmt.Errorf("action not configured")
)

type Actions interface {
	HasInit() bool
	DoInit(context.Context, params.Params) error

	HasReady() bool
	DoReady(context.Context, params.Params) error

	HasReset() bool
	DoReset(context.Context, params.Params) error
}

func CreateActions(config *Config) (Actions, error) {

	a := &actions{}

	if config.Ready != nil {
		action, err := config.Ready.Create()
		if err != nil {
			return nil, err
		}
		a.ready = action
	}

	if config.Init != nil {
		action, err := config.Init.Create()
		if err != nil {
			return nil, err
		}
		a.init = action
	}

	if config.Reset != nil {
		action, err := config.Reset.Create()
		if err != nil {
			return nil, err
		}
		a.reset = action
	}

	return a, nil
}

type actions struct {
	ready Action
	init  Action
	reset Action
}

func (m *actions) HasReady() bool {
	return m.ready != nil
}
func (m *actions) DoReady(ctx context.Context, p params.Params) error {
	if !m.HasReady() {
		return m.runAction(ctx, newActionNoop(), p, "ready")
	}
	return m.runAction(ctx, m.ready, p, "ready")
}

func (m *actions) HasInit() bool {
	return m.init != nil
}
func (m *actions) DoInit(ctx context.Context, p params.Params) error {
	if !m.HasInit() {
		return m.runAction(ctx, newActionNoop(), p, "init")
	}
	return m.runAction(ctx, m.init, p, "init")
}

func (m *actions) HasReset() bool {
	return m.reset != nil
}

func (m *actions) DoReset(ctx context.Context, p params.Params) error {
	if !m.HasReset() {
		return m.runAction(ctx, newActionNoop(), p, "reset")
	}
	return m.runAction(ctx, m.reset, p, "reset")
}

func (m *actions) runAction(ctx context.Context, action Action, p params.Params, name string) error {
	return newActionRunner(ctx, action, p, name).Run()
}
