package testutil

import (
	"io/ioutil"
	"path"
	"testing"

	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/pool"
	"github.com/boz/ephemerald/types"
	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ID(t *testing.T) types.ID {
	id, err := types.NewID()
	assert.NoError(t, err)
	return id
}

func Log() logrus.FieldLogger {
	log := logrus.New()
	log.Level = logrus.DebugLevel
	return log
}

func RunPoolFromFile(t *testing.T, path string, fn func(params.Params)) {
	// WithPoolFromFile(t, path, func(pool ephemerald.Pool) {
	// 	item, err := pool.Checkout()
	// 	require.NoError(t, err)

	// 	if fn != nil {
	// 		fn(item)
	// 	}

	// 	assert.NotNil(t, item)
	// 	// pool.Return(item)
	// })
}

func WithPoolFromFile(t *testing.T, basename string, fn func(pool.Pool)) {

	// buf := ReadJSON(t, basename)

	// log := Log()

	// config, err := config.Parse(log, Emitter(), t.Name(), buf)
	// require.NoError(t, err)

	// pool, err := ephemerald.NewPool(config)
	// require.NoError(t, err)

	// defer func() {
	// 	assert.NoError(t, pool.Stop())
	// }()

	// require.NoError(t, pool.WaitReady())

	// if fn != nil {
	// 	fn(pool)
	// }
}

func ReadJSON(t *testing.T, fpath string) []byte {
	buf, err := ioutil.ReadFile(path.Join("_testdata", fpath))
	require.NoError(t, err, fpath)
	if path.Ext(fpath) == ".yaml" {
		buf, err = yaml.YAMLToJSON(buf)
		require.NoError(t, err, fpath)
	}
	return buf
}
