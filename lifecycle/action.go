package lifecycle

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/boz/ephemerald/params"
)

const (
	ActionDefaultRetries = 3
	ActionDefaultTimeout = 1 * time.Second
	ActionDefaultDelay   = 500 * time.Millisecond
)

type Generator interface {
	Create() (Action, error)
}

type Action interface {
	Config() ActionConfig
	Do(Env, params.Params) error
}

type ActionConfig struct {
	Type    string        `json:"type"`
	Retries int           `json:"retries"`
	Timeout time.Duration `json:"timeout"`
	Delay   time.Duration `json:"delay"`
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
			return parseError("timeout", err)
		}
		ac.Timeout = val
	}

	if other.Delay != "" {
		val, err := time.ParseDuration(other.Delay)
		if err != nil {
			return parseError("delay", err)
		}
		ac.Delay = val
	}

	return nil
}

func parseError(field string, err error) error {
	return fmt.Errorf("error parsing field '%v': %v", field, err)
}
