package config_test

import (
	"testing"

	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/testutil"
	"github.com/boz/ephemerald/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRead(t *testing.T) {
	doReadTest(t, "_testdata/config.json", "json")
	doReadTest(t, "_testdata/config.yaml", "yaml")
	doReadTest(t, "_testdata/config.yml", "yml")
}

func doReadTest(t *testing.T, path string, msg string) {
	log := testutil.Log()

	configs, err := config.ReadFile(log, "_testdata/config.json")
	require.NoError(t, err, msg)

	require.Equal(t, 1, len(configs), msg)

	cfg := configs[0]

	assert.Equal(t, "redis", cfg.Name, msg)
	assert.Equal(t, "redis", cfg.Image, msg)
	assert.Equal(t, 6379, cfg.Port, msg)
	assert.Equal(t, 10, cfg.Size, msg)

	m := cfg.Actions.ForInstance(types.Instance{
		ID:     testutil.ID(t),
		PoolID: testutil.ID(t),
		Port:   cfg.Port,
		Host:   "127.0.0.1",
	})

	assert.False(t, m.HasInitialize(), msg)
	assert.True(t, m.HasHealthcheck(), msg)
	assert.True(t, m.HasReset(), msg)
}
