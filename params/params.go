package params

import (
	"bytes"
	"errors"
	"sync"
	"sync/atomic"
	"text/template"
)

var NotFoundError = errors.New("key not found")

type Params interface {
	Get(string) (string, error)
	Render(string) (string, error)
	RenderTemplate(*template.Template) (string, error)

	State() State
	ID() string
	Host() string
	Port() string

	Config() Config
	MergeConfig(Config) Params
	Merge(Params) Params

	// MergeVars(Params) Params
	// WithState(State) Params
	// Clone() Params
}

func New() Params {
	return &params{
		rctx: renderContext{
			templates: make(map[string]*ptemplate),
			values:    make(map[string]string),
			mtx:       sync.Mutex{},
		},
	}
}

func Create(state State, config Config) Params {
	params := &params{
		rctx: renderContext{
			State:     state,
			templates: make(map[string]*ptemplate),
			values:    make(map[string]string),
			mtx:       sync.Mutex{},
		},
	}
	for k, v := range config {
		params.rctx.templates[k] = &ptemplate{text: v}
	}
	return params
}

type params struct {
	rctx renderContext
	mtx  sync.Mutex
}

type renderContext struct {
	State
	templates map[string]*ptemplate
	values    map[string]string
	mtx       sync.Mutex
}

type ptemplate struct {
	text       string
	evaluating int32
}

func (p *params) State() State {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	return p.rctx.State
}

func (p *params) ID() string   { return string(p.State().ID) }
func (p *params) Host() string { return p.State().Host }
func (p *params) Port() string { return p.State().Port }

func (p *params) RenderTemplate(tmpl *template.Template) (string, error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	return p.rctx.renderTemplate(tmpl)
}

func (p *params) Render(text string) (string, error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	return p.rctx.render(text)
}

func (p *params) Get(key string) (string, error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	return p.rctx.Get(key)
}

func (p *params) Config() Config {
	cfg := make(map[string]string, len(p.rctx.templates))

	for k, v := range p.rctx.templates {
		cfg[k] = v.text
	}
	return cfg
}

func (p *params) MergeConfig(cfg Config) Params {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	next := &params{
		rctx: renderContext{
			State:     p.rctx.State,
			templates: make(map[string]*ptemplate),
			values:    make(map[string]string),
			mtx:       sync.Mutex{},
		},
	}

	for k, v := range p.rctx.templates {
		next.rctx.templates[k] = &ptemplate{text: v.text}
	}

	for k, v := range p.rctx.values {
		next.rctx.values[k] = v
	}

	for k, v := range cfg {
		if _, ok := next.rctx.templates[k]; !ok {
			next.rctx.templates[k] = &ptemplate{text: v}
		}
	}

	return next
}

func (p *params) Merge(other Params) Params {
	them, ok := other.(*params)
	if !ok {
		return p.MergeConfig(other.Config())
	}

	p.mtx.Lock()
	defer p.mtx.Unlock()

	next := &params{
		rctx: renderContext{
			State:     p.rctx.State,
			templates: make(map[string]*ptemplate),
			values:    make(map[string]string),
			mtx:       sync.Mutex{},
		},
	}

	for k, v := range p.rctx.templates {
		next.rctx.templates[k] = &ptemplate{text: v.text}
	}

	for k, v := range p.rctx.values {
		next.rctx.values[k] = v
	}

	for k, v := range them.rctx.templates {
		if _, ok := next.rctx.templates[k]; !ok {
			next.rctx.templates[k] = &ptemplate{text: v.text}
		} else {
			continue
		}

		if v, ok := them.rctx.values[k]; ok {
			next.rctx.values[k] = v
		}
	}

	return next

}

func (p *renderContext) Get(key string) (string, error) {
	var template *ptemplate

	p.mtx.Lock()
	if val, ok := p.values[key]; ok {
		p.mtx.Unlock()
		return val, nil
	}

	template, ok := p.templates[key]
	if !ok {
		p.mtx.Unlock()
		return "", errors.New("key not found: " + key)
	}

	p.mtx.Unlock()

	if !atomic.CompareAndSwapInt32(&template.evaluating, 0, 1) {
		return "", errors.New("cyclical dependency found: " + key)
	}

	val, err := p.render(template.text)
	if err != nil {
		return "", err
	}

	p.mtx.Lock()
	defer p.mtx.Unlock()
	p.values[key] = val
	return val, nil
}

func (p *renderContext) renderTemplate(tmpl *template.Template) (string, error) {
	buf := new(bytes.Buffer)
	err := tmpl.Execute(buf, p)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (p *renderContext) render(text string) (string, error) {
	tmpl, err := template.New("params-interpolate").Parse(text)
	if err != nil {
		return "", err
	}
	return p.renderTemplate(tmpl)
}

// func (p *Params) Reset() {
// 	p.mtx.Lock()
// 	defer p.mtx.Unlock()

// 	p.values = make(map[string]string)
// 	for _, val := range p.templates {
// 		atomic.StoreInt32(&val.evaluating, 0)
// 	}
// }

// func (p *Params) Merge(other Params) {
// }

// type Set map[string]Params

// TODO: move these.
// TODO: UDP

/*

username: u{{randhex 8}}
password: {{randhex 8}}
url: {{.Get "username"}}:{{.Get "password"}}@{{.Host}}:{{.Port}}

env:
	- PG_USERNAME={{.Get "username"}}
	- PG_PASSWORD={{.Get "password"}}

pre-exec:
- id
- host
- reset-count

- user-args:
- postgres-username
- postgres-database

post-exec:
- port
- docker-id
*/
