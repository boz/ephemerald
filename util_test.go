package cleanroom_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/ovrclk/cleanroom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTCPPorts(t *testing.T) {
	file, err := os.Open("testdata/inspect.postgres.json")
	require.NoError(t, err)
	defer file.Close()

	var status types.ContainerJSON

	require.NoError(t, json.NewDecoder(file).Decode(&status))

	ports := cleanroom.TCPPortsFor(status)

	assert.Equal(t, 1, len(ports))

	assert.Equal(t, "32768", ports["5432"])
}
