package postgres

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/boz/ephemerald/lifecycle"
	"github.com/boz/ephemerald/params"
	"github.com/buger/jsonparser"
)

func init() {
	lifecycle.MakeActionPlugin("postgres.truncate", actionPGTruncateParse)
}

func actionPGTruncateParse(buf []byte) (lifecycle.Generator, error) {
	action := &actionPGTruncate{
		ActionConfig: lifecycle.ActionConfig{
			Retries: defaultRetries,
			Timeout: defaultTimeout,
			Delay:   defaultDelay,
		},
	}
	err := json.Unmarshal(buf, action)

	if err != nil {
		return nil, err
	}

	{
		buf, dt, _, err := jsonparser.Get(buf, "exclude")
		switch {
		case err == nil:
			switch dt {
			case jsonparser.Array:
				err = json.Unmarshal(buf, &action.Exclude)
				if err != nil {
					return nil, err
				}
			case jsonparser.String:
				action.Exclude = []string{string(buf)}
			default:
				return nil, fmt.Errorf("postgres.truncate: exclude: bad type")
			}
		case err == jsonparser.KeyPathNotFoundError:
		default:
			return nil, err
		}
	}

	return action, nil
}

type actionPGTruncate struct {
	lifecycle.ActionConfig
	Exclude []string
}

func (a *actionPGTruncate) Create() (lifecycle.Action, error) {
	return &(*a), nil
}

func (a *actionPGTruncate) Do(e lifecycle.Env, p params.Params) error {
	db, err := openDB(e, p)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		e.Log().WithError(err).Debug("ERROR: begin")
		return err
	}

	rows, err := tx.QueryContext(e.Context(), "SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname = 'public'")

	if err != nil {
		tx.Rollback()
		e.Log().WithError(err).Debug("ERROR: select tables")
		return err
	}

	defer rows.Close()

rows:
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			tx.Rollback()
			e.Log().WithError(err).Debug("ERROR: scan")
			return err
		}

		for _, skipName := range a.Exclude {
			if strings.ToLower(skipName) == strings.ToLower(name) {
				continue rows
			}
		}

		if _, err := tx.ExecContext(e.Context(), "TRUNCATE TABLE "+name+" CASCADE"); err != nil {
			tx.Rollback()
			e.Log().WithError(err).Debug("ERROR: truncate")
			return err
		}
	}
	return tx.Commit()
}
