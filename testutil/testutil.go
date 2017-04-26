package testutil

import (
	"io/ioutil"
	"path"
	"strings"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald"
	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/ui"
	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func CID() string {
	return strings.Repeat("A", 36)
}

func Log() logrus.FieldLogger {
	log := logrus.New()
	log.Level = logrus.DebugLevel
	return log
}

func Emitter() ui.Emitter {
	return ui.NewNoopEmitter()
}

func PoolEmitter() ui.PoolEmitter {
	return Emitter().ForPool("pool-name")
}

func ContainerEmitter() ui.ContainerEmitter {
	return PoolEmitter().ForContainer(CID())
}

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

	buf := ReadJSON(t, basename)

	log := Log()

	config, err := config.Parse(log, Emitter(), t.Name(), buf)
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

func ReadJSON(t *testing.T, fpath string) []byte {
	buf, err := ioutil.ReadFile(path.Join("_testdata", fpath))
	require.NoError(t, err, fpath)
	if path.Ext(fpath) == ".yaml" {
		buf, err = yaml.YAMLToJSON(buf)
		require.NoError(t, err, fpath)
	}
	return buf
}
