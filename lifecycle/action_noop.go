package lifecycle

import (
	"encoding/json"

	"github.com/boz/ephemerald/params"
)

func init() {
	MakeActionPlugin("noop", actionNoopParse)
}

type actionNoop struct {
	ActionConfig
}

func actionNoopParse(buf []byte) (Action, error) {
	ac := &actionNoop{}
	return ac, json.Unmarshal(buf, ac)
}

func (a *actionNoop) Do(e Env, p params.Params) error {
	return nil
}
