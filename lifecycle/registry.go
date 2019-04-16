package lifecycle

import "fmt"

var (
	actionPlugins = map[string]ActionPlugin{}
)

type ActionPlugin interface {
	Name() string
	ParseConfig([]byte) (Generator, error)
}

func MakeActionPlugin(name string, fn func(buf []byte) (Generator, error)) {
	RegisterActionPlugin(&actionPlugin{name, fn})
}

func RegisterActionPlugin(ap ActionPlugin) {
	actionPlugins[ap.Name()] = ap
}

func lookupPlugin(name string) (ActionPlugin, error) {
	ap, ok := actionPlugins[name]
	if !ok {
		return nil, fmt.Errorf("action plugin '%v' not found", name)
	}
	return ap, nil
}

type actionPlugin struct {
	name        string
	parseConfig func([]byte) (Generator, error)
}

func (a *actionPlugin) Name() string {
	return a.name
}
func (a *actionPlugin) ParseConfig(buf []byte) (Generator, error) {
	return a.parseConfig(buf)
}
