package config

import (
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAll(t *testing.T) {
	js := []byte(`
		{
			"pools": {
				"redis": {
					"size": 10,
					"image": "redis",
					"port": 6379,

					"params": {
						"database": "0",
						"url": "redis://{{.Hostname}}:{{.Port}}/{{.Database}}"
					},

					"actions": {
						"healthcheck": {
							"type": "noop"
						},
						"reset": {
							"type": "noop"
						}
					}
				}
			}
		}
	`)

	log := logrus.New()
	configs, err := ParseAll(log, js)
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
