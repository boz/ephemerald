package lifecycle

import (
	"context"
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald/params"
	"github.com/buger/jsonparser"
)

var (
	ErrActionNotConfigured = fmt.Errorf("action not configured")
)

type Manager interface {
	HasInitialize() bool
	DoInitialize(context.Context, params.Params) error

	HasHealthcheck() bool
	DoHealthcheck(context.Context, params.Params) error

	HasReset() bool
	DoReset(context.Context, params.Params) error

	ParseConfig([]byte) error

	MaxDelay() time.Duration
	ForContainer(string) Manager
}

type manager struct {
	initializeAction  Action
	healthcheckAction Action
	resetAction       Action

	log logrus.FieldLogger
}

func NewManager(log logrus.FieldLogger) Manager {
	return &manager{log: log.WithField("component", "lifecycle.Manager")}
}

func (m *manager) ParseConfig(buf []byte) error {
	{
		action, err := m.parseAction(buf, "initialize")
		if err != nil {
			return err
		}
		m.initializeAction = action
	}
	{
		action, err := m.parseAction(buf, "healthcheck")
		if err != nil {
			return err
		}
		m.healthcheckAction = action
	}
	{
		action, err := m.parseAction(buf, "reset")
		if err != nil {
			return err
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
		return nil, fmt.Errorf("lifecycle manager: invalid config at %v: ", key, vt)
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

func (m *manager) HasInitialize() bool {
	return m.initializeAction != nil
}

func (m *manager) DoInitialize(ctx context.Context, p params.Params) error {
	if !m.HasInitialize() {
		return ErrActionNotConfigured
	}
	return newActionRunner(ctx, m.log, m.initializeAction, p, "initialize").Run()
}

func (m *manager) HasHealthcheck() bool {
	return m.healthcheckAction != nil
}

func (m *manager) DoHealthcheck(ctx context.Context, p params.Params) error {
	if !m.HasHealthcheck() {
		return ErrActionNotConfigured
	}
	return newActionRunner(ctx, m.log, m.healthcheckAction, p, "healthcheck").Run()
}

func (m *manager) HasReset() bool {
	return m.resetAction != nil
}

func (m *manager) DoReset(ctx context.Context, p params.Params) error {
	if !m.HasReset() {
		return ErrActionNotConfigured
	}
	return newActionRunner(ctx, m.log, m.resetAction, p, "reset").Run()
}

func (m *manager) ForContainer(id string) Manager {
	next := &(*m)
	next.log = m.log.WithField("container", id[0:12])
	return next
}
