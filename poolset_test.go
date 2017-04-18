package ephemerald_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald"
	"github.com/boz/ephemerald/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPoolSet(t *testing.T) {
	log := logrus.New()
	log.Level = logrus.DebugLevel
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	configs, err := config.ReadPath(log, "_testdata/pools.json")
	require.NoError(t, err)

	set, err := ephemerald.NewPoolSet(log, ctx, configs)
	require.NoError(t, err)
	defer set.Stop()

	require.NoError(t, set.WaitReady())

	pset, err := set.Checkout()
	require.NoError(t, err)
	defer set.ReturnAll(pset)

	rparam, ok := pset["redis"]
	if assert.True(t, ok) &&
		assert.NotNil(t, rparam) &&
		assert.NotEmpty(t, rparam.Url) {
	}

	pgparam, ok := pset["postgres"]
	if assert.True(t, ok) &&
		assert.NotNil(t, pgparam) &&
		assert.NotEmpty(t, pgparam.Url) {
	}
}
