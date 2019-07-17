package params

import (
	"bytes"
	"errors"
	"fmt"
	"text/template"

	"github.com/boz/ephemerald/types"
)

type RenderContext interface {
	InstanceID() types.ID
	PoolID() types.ID
	NumResets() int
	MaxResets() int
	Host() string
	Port() int
	Var(key string) (string, error)
}

type Params interface {
	RenderContext

	Render(string) (string, error)
	RenderTemplate(*template.Template) (string, error)
	MergeVars(map[string]string) Params
	ToCheckout() (*types.Checkout, error)
}

var NotFoundError = errors.New("key not found")

type actionContext struct {
	instanceID types.ID
	poolID     types.ID
	numResets  int
	maxResets  int
	host       string
	port       int
	vars       map[string]string
}

func Create(i types.Instance, vars map[string]string) Params {

	ac := &actionContext{
		instanceID: i.ID,
		poolID:     i.PoolID,
		numResets:  i.Resets,
		maxResets:  i.MaxResets,
		host:       i.Host,
		port:       i.Port,
		vars:       make(map[string]string, len(vars)),
	}

	for k, v := range vars {
		ac.vars[k] = v
	}

	return ac
}

func (ac *actionContext) InstanceID() types.ID { return ac.instanceID }
func (ac *actionContext) PoolID() types.ID     { return ac.poolID }
func (ac *actionContext) NumResets() int       { return ac.numResets }
func (ac *actionContext) MaxResets() int       { return ac.maxResets }
func (ac *actionContext) Host() string         { return ac.host }
func (ac *actionContext) Port() int            { return ac.port }

func (ac *actionContext) Var(name string) (string, error) {
	return newRenderContext(ac).Var(name)
}

func (ac *actionContext) Render(value string) (string, error) {
	return newRenderContext(ac).render(value)
}

func (ac *actionContext) RenderTemplate(tmpl *template.Template) (string, error) {
	return newRenderContext(ac).renderTemplate(tmpl)
}

func (ac actionContext) MergeVars(vars map[string]string) Params {
	mvars := make(map[string]string, len(ac.vars)+len(vars))

	for k, v := range ac.vars {
		mvars[k] = v
	}

	for k, v := range vars {
		if _, ok := mvars[k]; !ok {
			mvars[k] = v
		}
	}

	ac.vars = mvars

	return &ac
}

func (ac *actionContext) ToCheckout() (*types.Checkout, error) {
	co := &types.Checkout{
		InstanceID: ac.instanceID,
		PoolID:     ac.poolID,
		Host:       ac.host,
		Port:       ac.port,
		Vars:       make(map[string]string, len(ac.vars)),
	}

	rc := newRenderContext(ac)

	for k := range ac.vars {
		v, err := rc.Var(k)
		if err != nil {
			return nil, err
		}
		co.Vars[k] = v
	}

	return co, nil
}

func (ac actionContext) clone() actionContext {
	vars := make(map[string]string, len(ac.vars))
	for k, v := range ac.vars {
		vars[k] = v
	}
	ac.vars = vars
	return ac
}

func newRenderContext(ac *actionContext) *renderContext {
	return &renderContext{
		ac:    ac.clone(),
		rvars: make(map[string]bool, len(ac.vars)),
	}
}

type renderContext struct {
	ac    actionContext
	rvars map[string]bool
}

func (rc *renderContext) InstanceID() types.ID { return rc.ac.instanceID }
func (rc *renderContext) PoolID() types.ID     { return rc.ac.poolID }
func (rc *renderContext) NumResets() int       { return rc.ac.numResets }
func (rc *renderContext) MaxResets() int       { return rc.ac.maxResets }
func (rc *renderContext) Host() string         { return rc.ac.host }
func (rc *renderContext) Port() int            { return rc.ac.port }

func (rc *renderContext) Var(key string) (string, error) {
	if rc.rvars[key] {
		return "", errors.New("cyclical dependency")
	}
	rc.rvars[key] = true
	defer func() { rc.rvars[key] = false }()

	raw, ok := rc.ac.vars[key]
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
