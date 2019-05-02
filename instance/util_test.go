package instance

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTCPPorts(t *testing.T) {
	buf, err := ioutil.ReadFile(path.Join("_testdata", "inspect.postgres.json"))
	require.NoError(t, err)

	var status types.ContainerJSON

	require.NoError(t, json.Unmarshal(buf, &status))

	ports := tcpPortsFor(status)
	assert.Equal(t, 1, len(ports))

	assert.Equal(t, 32768, ports["5432"])
}
