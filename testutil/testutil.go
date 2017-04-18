package testutil

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald"
	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/params"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func RunPoolFromFile(t *testing.T, path string, fn func(params.Params)) {
	WithPoolFromFile(t, path, func(pool ephemerald.Pool) {
		item, err := pool.Checkout()
		require.NoError(t, err)

		if fn != nil {
			fn(item)
		}

		assert.NotNil(t, item)
		pool.Return(item)
	})
}

func WithPoolFromFile(t *testing.T, basename string, fn func(ephemerald.Pool)) {

	path := path.Join("_testdata", basename)

	log := logrus.New()
	log.Level = logrus.DebugLevel

	file, err := os.Open(path)
	require.NoError(t, err)
	buf, err := ioutil.ReadAll(file)
	require.NoError(t, err)

	config, err := config.Parse(log, t.Name(), buf)
	require.NoError(t, err)

	pool, err := ephemerald.NewPool(config)
	require.NoError(t, err)

	defer func() {
		assert.NoError(t, pool.Stop())
	}()

	require.NoError(t, pool.WaitReady())

	if fn != nil {
		fn(pool)
	}
}
