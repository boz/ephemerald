package params

import (
	"bytes"
	"net/url"
	"strconv"
	"text/template"

	"github.com/boz/ephemerald/types"
	dtypes "github.com/docker/docker/api/types"
)

type Params struct {
	ID   string `json:"id"`
	Host string `json:"host"`
	Port string `json:"port"`
}

type Set map[string]Params

func ParamsFor(c types.Container, status dtypes.ContainerJSON, port int) (Params, error) {
	return Params{
		ID:   string(c.ID),
		Host: c.Host,
		Port: TCPPortFor(status, port),
	}, nil
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

func (p Params) queryEscape() Params {
	return Params{
		ID:   url.QueryEscape(p.ID),
		Host: url.QueryEscape(p.Host),
		Port: url.QueryEscape(p.Port),
	}
}

// TODO: move these.
// TODO: UDP

func TCPPortFor(status dtypes.ContainerJSON, port int) string {
	ports := TCPPortsFor(status)
	return ports[strconv.Itoa(port)]
}

func TCPPortsFor(status dtypes.ContainerJSON) map[string]string {
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
