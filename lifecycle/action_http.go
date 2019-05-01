package lifecycle

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/boz/ephemerald/params"
	"github.com/buger/jsonparser"
)

func init() {
	MakeActionPlugin("http.get", actionHttpGetParse)
}

type actionHttpGet struct {
	ActionConfig
	Url string
}

func actionHttpGetParse(buf []byte) (Generator, error) {
	action := &actionHttpGet{
		ActionConfig: DefaultActionConfig(),
	}

	if err := json.Unmarshal(buf, action); err != nil {
		return nil, err
	}

	{
		val, err := jsonparser.GetString(buf, "url")
		switch {
		case err == nil:
			action.Url = val
		case err == jsonparser.KeyPathNotFoundError:
			action.Url = "http://{{.Host}:{{.Port}}"
		default:
			return nil, err
		}
	}

	return action, nil
}

func (a *actionHttpGet) Create() (Action, error) {
	return &(*a), nil
}

func (a *actionHttpGet) Do(e Env, p params.Params) error {

	p = p.MergeVars(map[string]string{"url": a.Url})

	url, err := p.Var("url")
	if err != nil {
		return err
	}

	if url == "" {
		return fmt.Errorf("http.get: no url found")
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req = req.WithContext(e.Context())

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(ioutil.Discard, resp.Body)

	return err
}
