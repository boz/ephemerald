package cpool

import (
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResource(t *testing.T) {
	resource := NewPGResource()
	pool, err := NewPool(resource, 5)
	require.NoError(t, err)
	defer pool.Stop()

	assert.NoError(t, pool.Fetch())

	require.NoError(t, pool.Stop())

}
func TestMain(m *testing.M) {
	logrus.SetLevel(logrus.DebugLevel)
}
