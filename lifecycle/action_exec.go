package lifecycle

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/boz/ephemerald/params"
	"github.com/buger/jsonparser"
)

const (
	actionExecDefaultTimeout = time.Second * 5
)

func init() {
	MakeActionPlugin("exec", actionExecParse)
}

type actionExec struct {
	ActionConfig
	Path string
	Args []string
	Env  []string
	Dir  string
}

func actionExecParse(buf []byte) (Action, error) {
	ac := DefaultActionConfig()
	ac.Timeout = actionExecDefaultTimeout

	action := &actionExec{
		ActionConfig: ac,
	}

	if err := json.Unmarshal(buf, action); err != nil {
		return nil, err
	}

	{
		val, err := jsonparser.GetString(buf, "path")
		switch {
		case err == nil:
			action.Path = val
		case err == jsonparser.KeyPathNotFoundError:
			return nil, fmt.Errorf("exec: no path given")
		default:
			return nil, err
		}
	}

	{
		buf, dt, _, err := jsonparser.Get(buf, "args")
		switch {
		case err == nil:
			switch dt {
			case jsonparser.Array:
				err = json.Unmarshal(buf, &action.Args)
				if err != nil {
					return nil, err
				}
			default:
				return nil, fmt.Errorf("exec: args bad type")
			}
		case err == jsonparser.KeyPathNotFoundError:
		default:
			return nil, err
		}
	}

	{
		buf, dt, _, err := jsonparser.Get(buf, "env")
		switch {
		case err == nil:
			switch dt {
			case jsonparser.Array:
				err = json.Unmarshal(buf, &action.Env)
				if err != nil {
					return nil, err
				}
			default:
				return nil, fmt.Errorf("exec: args bad type")
			}
		case err == jsonparser.KeyPathNotFoundError:
		default:
			return nil, err
		}
	}

	{
		val, err := jsonparser.GetString(buf, "dir")
		switch {
		case err == nil:
			action.Dir = val
		case err == jsonparser.KeyPathNotFoundError:
		default:
			return nil, err
		}
	}

	return action, nil
}

func (a *actionExec) Do(e Env, p params.Params) error {

	env := []string{
		fmt.Sprintf("EPHEMERALD_ID=%v", p.ID),
		fmt.Sprintf("EPHEMERALD_HOST=%v", p.Host),
		fmt.Sprintf("EPHEMERALD_PORT=%v", p.Port),
		// fmt.Sprintf("EPHEMERALD_USERNAME=%v", p.Username),
		// fmt.Sprintf("EPHEMERALD_PASSWORD=%v", p.Password),
		// fmt.Sprintf("EPHEMERALD_DATABASE=%v", p.Database),
		// fmt.Sprintf("EPHEMERALD_URL=%v", p.Url),
	}

	for _, text := range a.Env {
		val, err := p.Interpolate(text)
		if err != nil {
			// TODO: unrecoverable errors
			return err
		}
		env = append(env, val)
	}

	var wg sync.WaitGroup

	cmd := exec.CommandContext(e.Context(), a.Path, a.Args...)
	cmd.Dir = a.Dir
	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		copyLogs(stdout, e.Log().Debugln)
	}()

	go func() {
		defer wg.Done()
		copyLogs(stderr, e.Log().Errorln)
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}

func copyLogs(r io.Reader, logfn func(args ...interface{})) {
	buf := make([]byte, 80)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			logfn(buf[0:n])
		}
		if err != nil {
			break
		}
	}
}
