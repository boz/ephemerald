package lifecycle

import (
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseManager_full(t *testing.T) {
	js := []byte(`{
		"initialize": {
			"type": "noop"
		},
		"healthcheck": {
			"type": "noop"
		},
		"reset": {
			"type": "noop"
		}
	}`)

	log := logrus.New()

	m := NewManager(log)

	require.NoError(t, m.ParseConfig(js))

	assert.True(t, m.HasInitialize())
	assert.True(t, m.HasHealthcheck())
	assert.True(t, m.HasReset())
}

func TestParseManager_partial(t *testing.T) {
	js := []byte(`{
		"initialize": {
			"type": "noop"
		}
	}`)

	log := logrus.New()
	m := NewManager(log)

	require.NoError(t, m.ParseConfig(js))

	assert.True(t, m.HasInitialize())
	assert.False(t, m.HasHealthcheck())
	assert.False(t, m.HasReset())
}
