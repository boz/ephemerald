package instance

import (
	"strconv"

	dtypes "github.com/docker/docker/api/types"
)

func tcpPortFor(status dtypes.ContainerJSON, port int) string {
	ports := tcpPortsFor(status)
	return ports[strconv.Itoa(port)]
}

func tcpPortsFor(status dtypes.ContainerJSON) map[string]string {
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
