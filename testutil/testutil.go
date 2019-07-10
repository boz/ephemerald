package testutil

import (
	"context"
	"io/ioutil"
	"path"
	"testing"

	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/log"
	"github.com/boz/ephemerald/node"
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

func Context() context.Context {
	return log.NewContext(context.Background(), Log())
}

func Log() logrus.FieldLogger {
	l := logrus.New()
	l.SetLevel(logrus.DebugLevel)
	return l
}

func Bus(t *testing.T, ctx context.Context) pubsub.Service {
	bus, err := pubsub.NewBus(ctx)
	require.NoError(t, err)
	return bus
}

func Node(t *testing.T, ctx context.Context) node.Node {
	node, err := node.NewFromEnv(ctx)
	require.NoError(t, err)
	return node
}

func RunPoolFromFile(t *testing.T, path string) {
	WithCheckoutFromFile(t, path, func(_ *types.Checkout) {
	})
}

func WithCheckoutFromFile(t *testing.T, path string, fn func(*types.Checkout)) {
	WithPoolFromFile(t, path, func(pool pool.Pool) {
		ctx := context.Background()
		co, err := pool.Checkout(ctx)
		require.NoError(t, err)

		fn(co)

		assert.NoError(t, pool.Release(ctx, co.InstanceID))
	})
}

func WithPoolFromFile(t *testing.T, basename string, fn func(pool.Pool)) {
	ctx, cancel := context.WithCancel(Context())
	defer cancel()

	bus, err := pubsub.NewBus(ctx)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, bus.Shutdown())
	}()

	node, err := node.NewFromEnv(ctx)
	require.NoError(t, err)

	sched := scheduler.New(ctx, bus, node)

	var cfg config.Pool

	err = config.ReadFile(basename, &cfg)

	require.NoError(t, err)

	pool, err := pool.Create(ctx, bus, sched, cfg)
	require.NoError(t, err)

	defer func() {
		pool.Shutdown()
		<-pool.Done()
	}()

	if fn != nil {
		fn(pool)
	}
}

func ReadFile(t *testing.T, fpath string, obj interface{}) {
	require.NoError(t, config.ReadFile(fpath, obj))
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
