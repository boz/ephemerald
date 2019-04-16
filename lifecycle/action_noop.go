package lifecycle

import (
	"encoding/json"
	"time"

	"github.com/boz/ephemerald/params"
)

func init() {
	MakeActionPlugin("noop", actionNoopParse)
}

type actionNoop struct {
	ActionConfig
}

func newActionNoop() *actionNoop {
	return &actionNoop{
		ActionConfig: ActionConfig{
			Type:    "noop",
			Timeout: time.Second,
		},
	}
}

func actionNoopParse(buf []byte) (Generator, error) {
	ac := newActionNoop()
	return ac, json.Unmarshal(buf, ac)
}

func (a *actionNoop) Create() (Action, error) {
	return &(*a), nil
}

func (a *actionNoop) Do(e Env, p params.Params) error {
	return nil
}
