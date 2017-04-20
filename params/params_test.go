package params

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfig(t *testing.T) {
	buf, err := ioutil.ReadFile("_testdata/config.params.json")
	require.NoError(t, err)

	cfg, err := ParseConfig(buf)
	require.NoError(t, err)

	assert.Equal(t, "postgres", cfg.Username)
	assert.Equal(t, "", cfg.Password)
	assert.Equal(t, "postgres", cfg.Database)
}

func TestTCPPorts(t *testing.T) {
	file, err := os.Open("_testdata/inspect.postgres.json")
	require.NoError(t, err)
	defer file.Close()

	var status types.ContainerJSON

	require.NoError(t, json.NewDecoder(file).Decode(&status))

	ports := tcpPortsFor(status)
	assert.Equal(t, 1, len(ports))

	assert.Equal(t, "32768", ports["5432"])
}
