package cleanroom

import (
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
)

func lcid(log logrus.FieldLogger, id string) logrus.FieldLogger {
	return log.WithField("container", id[0:12])
}

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

	for port, _ := range status.Config.ExposedPorts {
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
