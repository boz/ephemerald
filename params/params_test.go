package params_test

import (
	"encoding/json"
	"testing"

	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/testutil"
	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// func TestParseConfig(t *testing.T) {

// 	bufs := map[string][]byte{
// 		"json": testutil.ReadJSON(t, "config.params.json"),
// 		"yaml": testutil.ReadJSON(t, "config.params.yaml"),
// 	}

// 	for ext, buf := range bufs {
// 		cfg, err := params.ParseConfig(buf)
// 		require.NoError(t, err, ext)
// 		assert.Equal(t, "postgres", cfg.Username, ext)
// 		assert.Equal(t, "", cfg.Password, ext)
// 		assert.Equal(t, "postgres", cfg.Database, ext)
// 	}
// }

func TestTCPPorts(t *testing.T) {
	buf := testutil.ReadJSON(t, "inspect.postgres.json")

	var status types.ContainerJSON

	require.NoError(t, json.Unmarshal(buf, &status))

	ports := params.TCPPortsFor(status)
	assert.Equal(t, 1, len(ports))

	assert.Equal(t, "32768", ports["5432"])
}
