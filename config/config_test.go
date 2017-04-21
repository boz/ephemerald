package config_test

import (
	"io/ioutil"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAll(t *testing.T) {
	buf, err := ioutil.ReadFile("_testdata/config.json")
	require.NoError(t, err)

	log := logrus.New()
	configs, err := config.ParseAll(log, testutil.Emitter(), buf)
	require.NoError(t, err)
	require.Equal(t, 1, len(configs))

	cfg := configs[0]

	assert.Equal(t, "redis", cfg.Name)
	assert.Equal(t, "redis", cfg.Image)
	assert.Equal(t, 6379, cfg.Port)
	assert.Equal(t, 10, cfg.Size)

	m := cfg.Lifecycle.ForContainer(testutil.ContainerEmitter(), testutil.CID())

	assert.False(t, m.HasInitialize())
	assert.True(t, m.HasHealthcheck())
	assert.True(t, m.HasReset())
}
