package params

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strconv"
	"text/template"

	"github.com/docker/docker/api/types"
)

const (

	// hostname of where containers are running
	defaultHostname = "localhost"
)

// hard-code parameters and url generation for now.

type Config struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Database string `json:"database,omitempty"`
	Url      string `json:"url,omitempty"`

	urlTemplate *template.Template
}

func ParseConfig(buf []byte) (Config, error) {
	cfg := Config{}
	err := json.Unmarshal(buf, &cfg)
	if err != nil {
		return cfg, err
	}

	tmpl, err := template.New("config-url").Parse(cfg.Url)
	if err != nil {
		return cfg, err
	}

	cfg.urlTemplate = tmpl

	return cfg, nil
}

func (c Config) ParamsFor(id string, status types.ContainerJSON, port int) (Params, error) {
	p := Params{
		Config: c,
		Id:     id,
		Port:   TCPPortFor(status, port),
	}
	return p.ForHost(defaultHostname)
}

type Params struct {
	Id       string `json:"id"`
	Hostname string `json:"hostname"`
	Port     string `json:"port"`
	Config
}

type Set map[string]Params

func (p Params) ID() string {
	return p.Id
}

func (p Params) ForHost(host string) (Params, error) {
	p.Hostname = host
	url, err := p.generateURL()
	p.Url = url
	return p, err
}

func (p Params) ExecuteTemplate(tmpl *template.Template) (string, error) {
	buf := new(bytes.Buffer)
	err := tmpl.Execute(buf, p)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (p Params) Interpolate(text string) (string, error) {
	tmpl, err := template.New("params-interpolate").Parse(text)
	if err != nil {
		return "", err
	}
	return p.ExecuteTemplate(tmpl)
}

func (p Params) generateURL() (string, error) {
	return p.queryEscape().ExecuteTemplate(p.urlTemplate)
}

func (p Params) queryEscape() Params {
	return Params{
		Id:       url.QueryEscape(p.Id),
		Hostname: url.QueryEscape(p.Hostname),
		Port:     url.QueryEscape(p.Port),
		Config: Config{
			Username: url.QueryEscape(p.Username),
			Password: url.QueryEscape(p.Password),
			Database: url.QueryEscape(p.Database),
		},
	}
}

// TODO: move these.

func TCPPortFor(status types.ContainerJSON, port int) string {
	ports := TCPPortsFor(status)
	return ports[strconv.Itoa(port)]
}

func TCPPortsFor(status types.ContainerJSON) map[string]string {
	ports := make(map[string]string)

	if status.Config == nil {
		return ports
	}
	if status.NetworkSettings == nil {
		return ports
	}

	for port := range status.Config.ExposedPorts {
		if port.Proto() != "tcp" {
			continue
		}
		eport, ok := status.NetworkSettings.Ports[port]
		if !ok || len(eport) == 0 {
			continue
		}
		ports[port.Port()] = eport[0].HostPort
	}

	return ports
}
