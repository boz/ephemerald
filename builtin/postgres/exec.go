package postgres

import (
	"encoding/json"
	"fmt"

	"github.com/boz/ephemerald/lifecycle"
	"github.com/boz/ephemerald/params"
	"github.com/buger/jsonparser"
)

const (
	defaultRetries = 10
	defaultTimeout = lifecycle.ActionDefaultTimeout
	defaultDelay   = lifecycle.ActionDefaultDelay
)

func init() {
	lifecycle.MakeActionPlugin("postgres.exec", actionPGExecParse)
}

func actionPGExecParse(buf []byte) (lifecycle.Generator, error) {
	action := &actionPGExec{
		ActionConfig: lifecycle.ActionConfig{
			Retries: defaultRetries,
			Timeout: defaultTimeout,
			Delay:   defaultDelay,
		},
		Query: "SELECT 1 = 1",
	}
	err := json.Unmarshal(buf, action)
	if err != nil {
		return nil, err
	}

	{
		val, err := jsonparser.GetString(buf, "query")
		switch {
		case err == nil:
			action.Query = val
		case err == jsonparser.KeyPathNotFoundError:
		default:
			return nil, err
		}
	}

	{
		buf, dt, _, err := jsonparser.Get(buf, "params")
		switch {
		case err == nil:
			if dt != jsonparser.Array {
				return nil, fmt.Errorf("postgres.exec: bad params type")
			}
			err = json.Unmarshal(buf, action.Params)
			if err != nil {
				return nil, err
			}
		case err == jsonparser.KeyPathNotFoundError:
		default:
			return nil, err
		}
	}

	return action, nil
}

type actionPGExec struct {
	lifecycle.ActionConfig
	pgParams
	Query  string
	Params []string
}

func (a actionPGExec) Create() (lifecycle.Action, error) {
	return &a, nil
}

func (a *actionPGExec) Do(e lifecycle.Env, p params.Params) error {

	p = params.MergeDefaultsWithOverride(p, a.pgParams.ParamConfig(), defaultParamConfig())

	db, err := openDB(e, p)
	if err != nil {
		return err
	}
	defer db.Close()

	args := make([]interface{}, 0, len(a.Params))
	for _, arg := range a.Params {
		args = append(args, arg)
	}

	e.Log().WithField("query", a.Query).Debug("initializing")

	_, err = db.ExecContext(e.Context(), a.Query, args...)
	if err != nil {
		e.Log().WithError(err).Debug("ERROR: executing")
	}

	return err
}
