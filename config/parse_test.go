package config_test

import (
	"testing"

	"github.com/boz/ephemerald/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRead(t *testing.T) {
	doReadTest(t, "_testdata/config.json", "json")
	doReadTest(t, "_testdata/config.yaml", "yaml")
	doReadTest(t, "_testdata/config.yml", "yml")
}

func doReadTest(t *testing.T, path string, msg string) {
	cfg := config.Pool{}

	err := config.ReadFile("_testdata/config.json", &cfg)
	require.NoError(t, err, msg)

	assert.Equal(t, "redis", cfg.Name, msg)
	assert.Equal(t, "redis", cfg.Image, msg)
	assert.Equal(t, 6379, cfg.Port, msg)
	assert.Equal(t, 10, cfg.Size, msg)

	assert.NotNil(t, cfg.Params)
	assert.Equal(t, "bar", cfg.Params["foo"])

	assert.Nil(t, cfg.Actions.Init, msg)
	assert.NotNil(t, cfg.Actions.Live, msg)
	assert.NotNil(t, cfg.Actions.Reset, msg)
}
