package lifecycle

import (
	"context"
	"fmt"
	"time"

	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/types"
	"github.com/buger/jsonparser"
	"github.com/sirupsen/logrus"
)

var (
	ErrActionNotConfigured = fmt.Errorf("action not configured")
)

type Manager interface {
	ParseConfig([]byte) error
	MaxDelay() time.Duration
	ForInstance(types.Instance) ContainerManager
}

// StartedProbe
// ReadyProbe

type ContainerManager interface {
	HasInitialize() bool
	DoInitialize(context.Context, params.Params) error

	HasHealthcheck() bool
	DoHealthcheck(context.Context, params.Params) error

	HasReset() bool
	DoReset(context.Context, params.Params) error
}

type manager struct {
	initializeAction  Action
	healthcheckAction Action
	resetAction       Action

	log logrus.FieldLogger
}

type containerManager struct {
	manager
}

func NewManager(log logrus.FieldLogger) Manager {
	return &manager{log: log.WithField("cmp", "lifecycle.manager")}
}

func (m *manager) ForInstance(i types.Instance) ContainerManager {
	next := &containerManager{
		manager: *m,
	}
	next.log = m.log.
		WithField("pid", i.PoolID).
		WithField("iid", i.ID)
	return next
}

func (m *manager) ParseConfig(buf []byte) error {
	{
		action, err := m.parseAction(buf, "initialize")
		if err != nil {
			return parseError("initialize", err)
		}
		m.initializeAction = action
	}
	{
		action, err := m.parseAction(buf, "healthcheck")
		if err != nil {
			return parseError("healthcheck", err)
		}
		m.healthcheckAction = action
	}
	{
		action, err := m.parseAction(buf, "reset")
		if err != nil {
			return parseError("reset", err)
		}
		m.resetAction = action
	}
	return nil
}

func (m *manager) parseAction(buf []byte, key string) (Action, error) {
	vbuf, vt, _, err := jsonparser.Get(buf, key)
	if vt == jsonparser.NotExist && err == jsonparser.KeyPathNotFoundError {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	switch vt {
	case jsonparser.Object:
		return ParseAction(vbuf)
	default:
		return nil, fmt.Errorf("lifecycle manager: invalid config at %v: ", key)
	}
}

func (m *manager) MaxDelay() time.Duration {
	max := time.Duration(0)
	for _, action := range m.actions() {
		cfg := action.Config()
		val := (cfg.Delay + cfg.Timeout) * time.Duration(cfg.Retries+1)
		if val > max {
			max = val
		}
	}
	return max
}

func (m *manager) actions() []Action {
	var actions []Action
	if m.initializeAction != nil {
		actions = append(actions, m.initializeAction)
	}
	if m.healthcheckAction != nil {
		actions = append(actions, m.healthcheckAction)
	}
	if m.resetAction != nil {
		actions = append(actions, m.resetAction)
	}
	return actions
}

func (m *containerManager) HasInitialize() bool {
	return m.initializeAction != nil
}

func (m *containerManager) DoInitialize(ctx context.Context, p params.Params) error {
	if !m.HasInitialize() {
		return m.runAction(ctx, &actionNoop{}, p, "initialize")
	}
	return m.runAction(ctx, m.initializeAction, p, "initialize")
}

func (m *containerManager) HasHealthcheck() bool {
	return m.healthcheckAction != nil
}

func (m *containerManager) DoHealthcheck(ctx context.Context, p params.Params) error {
	if !m.HasHealthcheck() {
		return m.runAction(ctx, &actionNoop{}, p, "healthcheck")
	}
	return m.runAction(ctx, m.healthcheckAction, p, "healthcheck")
}

func (m *containerManager) HasReset() bool {
	return m.resetAction != nil
}

func (m *containerManager) DoReset(ctx context.Context, p params.Params) error {
	if !m.HasReset() {
		return m.runAction(ctx, &actionNoop{}, p, "reset")
	}
	return m.runAction(ctx, m.resetAction, p, "reset")
}

func (m *containerManager) runAction(ctx context.Context, action Action, p params.Params, name string) error {
	return newActionRunner(ctx, m.log, action, p, name).Run()
}
