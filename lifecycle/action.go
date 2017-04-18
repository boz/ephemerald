package lifecycle

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/boz/ephemerald/params"
	"github.com/buger/jsonparser"
)

var (
	actionPlugins = map[string]ActionPlugin{}
)

const (
	ActionDefaultRetries = 3
	ActionDefaultTimeout = 1 * time.Second
	ActionDefaultDelay   = 500 * time.Millisecond
)

type Action interface {
	Config() ActionConfig
	Do(Env, params.Params) error
}

type ActionConfig struct {
	Type    string
	Retries int
	Timeout time.Duration
	Delay   time.Duration
}

func (ac ActionConfig) Config() ActionConfig {
	return ac
}

func DefaultActionConfig() ActionConfig {
	return ActionConfig{
		Retries: ActionDefaultRetries,
		Timeout: ActionDefaultTimeout,
		Delay:   ActionDefaultDelay,
	}
}

type ActionPlugin interface {
	Name() string
	ParseConfig([]byte) (Action, error)
}

func ParseAction(buf []byte) (Action, error) {
	t, err := jsonparser.GetString(buf, "type")
	if err != nil {
		return nil, err
	}

	p, err := lookupPlugin(t)
	if err != nil {
		return nil, err
	}
	return p.ParseConfig(buf)
}

func (ac *ActionConfig) UnmarshalJSON(buf []byte) error {
	other := struct {
		Type    string
		Retries int
		Timeout string
		Delay   string
	}{Retries: ac.Retries}

	err := json.Unmarshal(buf, &other)
	if err != nil {
		return err
	}

	ac.Type = other.Type
	ac.Retries = other.Retries

	if other.Timeout != "" {
		val, err := time.ParseDuration(other.Timeout)
		if err != nil {
			return err
		}
		ac.Timeout = val
	}

	if other.Delay != "" {
		val, err := time.ParseDuration(other.Delay)
		if err != nil {
			return err
		}
		ac.Delay = val
	}

	return nil
}

func MakeActionPlugin(name string, fn func(buf []byte) (Action, error)) {
	RegisterActionPlugin(&actionPlugin{name, fn})
}

func RegisterActionPlugin(ap ActionPlugin) {
	actionPlugins[ap.Name()] = ap
}

func lookupPlugin(name string) (ActionPlugin, error) {
	ap, ok := actionPlugins[name]
	if !ok {
		return nil, fmt.Errorf("action plugin '%v' not found", name)
	}
	return ap, nil
}

type actionPlugin struct {
	name        string
	parseConfig func([]byte) (Action, error)
}

func (a *actionPlugin) Name() string {
	return a.name
}
func (a *actionPlugin) ParseConfig(buf []byte) (Action, error) {
	return a.parseConfig(buf)
}
