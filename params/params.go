package params

import (
	"bytes"
	"errors"
	"fmt"
	"text/template"
)

var NotFoundError = errors.New("key not found")

type ActionContext struct {
	InstanceID ID                `json:"instance-id"`
	PoolID     ID                `json:"pool-id"`
	NumResets  int               `json:"num-resets"`
	MaxResets  int               `json:"max-resets"`
	Host       string            `json:"host"`
	Port       string            `json:"port"`
	Vars       map[string]string `json:"vars"`
}

func (ac *ActionContext) Var(name string) (string, error) {
	return newRenderContext(ac).Var(name)
}

func (ac *ActionContext) Render(value string) (string, error) {
	return newRenderContext(ac).render(value)
}

func (ac *ActionContext) RenderTemplate(tmpl *template.Template) (string, error) {
	return newRenderContext(ac).renderTemplate(tmpl)
}

func (ac ActionContext) clone() ActionContext {
	vars := make(map[string]string, len(ac.Vars))
	for k, v := range ac.Vars {
		vars[k] = v
	}
	ac.Vars = vars
	return ac
}

func newRenderContext(ac *ActionContext) *renderContext {
	return &renderContext{
		ac:    ac.clone(),
		rvars: make(map[string]bool, len(ac.Vars)),
	}
}

type renderContext struct {
	ac    ActionContext
	rvars map[string]bool
}

func (rc *renderContext) InstanceID() ID { return rc.ac.InstanceID }
func (rc *renderContext) PoolID() ID     { return rc.ac.PoolID }
func (rc *renderContext) NumResets() int { return rc.ac.NumResets }
func (rc *renderContext) MaxResets() int { return rc.ac.MaxResets }
func (rc *renderContext) Host() string   { return rc.ac.Host }
func (rc *renderContext) Port() string   { return rc.ac.Port }

func (rc *renderContext) Var(key string) (string, error) {
	if rc.rvars[key] {
		return "", errors.New("cyclical dependency")
	}
	rc.rvars[key] = true
	defer func() { rc.rvars[key] = false }()

	raw, ok := rc.ac.Vars[key]
	if !ok {
		return "", fmt.Errorf("unknown key: %v", key)
	}
	return rc.render(raw)
}

func (rc *renderContext) render(text string) (string, error) {
	tmpl, err := template.New("params-interpolate").Parse(text)
	if err != nil {
		return "", err
	}
	return rc.renderTemplate(tmpl)
}

func (rc *renderContext) renderTemplate(tmpl *template.Template) (string, error) {
	buf := new(bytes.Buffer)
	err := tmpl.Execute(buf, rc)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
