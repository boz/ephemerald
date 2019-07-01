package lifecycle

import (
	"context"
	"fmt"

	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/pubsub"
	"github.com/boz/ephemerald/types"
)

var (
	ErrActionNotConfigured = fmt.Errorf("action not configured")
)

type Actions interface {
	HasInit() bool
	DoInit(context.Context, params.Params) error

	HasLive() bool
	DoLive(context.Context, params.Params) error

	HasReset() bool
	DoReset(context.Context, params.Params) error
}

func CreateActions(instance types.Instance, bus pubsub.Bus, config *Config) (Actions, error) {

	a := &actions{instance: instance, bus: bus}

	if config.Live != nil {
		action, err := config.Live.Create()
		if err != nil {
			return nil, err
		}
		a.live = action
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
	instance types.Instance
	bus      pubsub.Bus
	live     Action
	init     Action
	reset    Action
}

func (m *actions) HasLive() bool {
	return m.live != nil
}
func (m *actions) DoLive(ctx context.Context, p params.Params) error {
	if !m.HasLive() {
		return m.runAction(ctx, newActionNoop(), p, "live")
	}
	return m.runAction(ctx, m.live, p, "live")
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
	return newActionRunner(m.bus, m.instance, ctx, action, p, name).Run()
}
