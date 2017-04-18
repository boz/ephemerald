package lifecycle

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"text/template"

	"github.com/boz/ephemerald/params"
	"github.com/buger/jsonparser"
)

func init() {
	MakeActionPlugin("http.get", actionHttpGetParse)
}

type actionHttpGet struct {
	ActionConfig
	Url  string
	tmpl *template.Template
}

func actionHttpGetParse(buf []byte) (Action, error) {
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
		default:
			return nil, err
		}
	}

	if action.Url != "" {
		tmpl, err := template.New("http-get-url").Parse(action.Url)
		if err != nil {
			return nil, err
		}
		action.tmpl = tmpl
	}

	return action, nil
}

func (a *actionHttpGet) Do(e Env, p params.Params) error {

	var url string
	var err error

	url = p.Url

	if a.tmpl != nil {
		url, err = p.ExecuteTemplate(a.tmpl)
		if err != nil {
			return err
		}
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
