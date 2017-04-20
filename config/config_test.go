package config

import (
	"io/ioutil"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAll(t *testing.T) {
	buf, err := ioutil.ReadFile("_testdata/config.json")
	require.NoError(t, err)

	log := logrus.New()
	configs, err := ParseAll(log, buf)
	require.NoError(t, err)
	require.Equal(t, 1, len(configs))

	cfg := configs[0]

	assert.Equal(t, "redis", cfg.Name)
	assert.Equal(t, "redis", cfg.Image)
	assert.Equal(t, 6379, cfg.Port)
	assert.Equal(t, 10, cfg.Size)

	assert.False(t, cfg.Lifecycle.HasInitialize())
	assert.True(t, cfg.Lifecycle.HasHealthcheck())
	assert.True(t, cfg.Lifecycle.HasReset())
}
