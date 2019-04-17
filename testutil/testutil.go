package testutil

import (
	"context"
	"io/ioutil"
	"path"
	"testing"

	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/log"
	"github.com/boz/ephemerald/node"
	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/pool"
	"github.com/boz/ephemerald/pubsub"
	"github.com/boz/ephemerald/scheduler"
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
	return log.New()
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus, err := pubsub.NewBus(ctx)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, bus.Shutdown())
	}()

	node, err := node.NewFromEnv(ctx)
	require.NoError(t, err)

	sched := scheduler.New(bus, node)

	var cfg config.Pool

	err = config.ReadFile(basename, &cfg)

	require.NoError(t, err)

	pool, err := pool.Create(ctx, bus, sched, cfg)
	require.NoError(t, err)

	defer func() {
		pool.Shutdown()
		<-pool.Done()
	}()

	// require.NoError(t, pool.WaitReady())

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
