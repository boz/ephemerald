package instance

import (
	"strconv"

	dtypes "github.com/docker/docker/api/types"
)

func tcpPortFor(status dtypes.ContainerJSON, port int) int {
	ports := tcpPortsFor(status)
	return ports[strconv.Itoa(port)]
}

func tcpPortsFor(status dtypes.ContainerJSON) map[string]int {
	ports := make(map[string]int)

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
		hport, err := strconv.Atoi(eport[0].HostPort)
		if err != nil {
			// XXX: handle error
			continue
		}
		ports[port.Port()] = hport
	}

	return ports
}
