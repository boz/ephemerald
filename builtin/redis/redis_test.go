package redis_test

import (
	"testing"

	"github.com/Sirupsen/logrus"
	rredis "github.com/garyburd/redigo/redis"
	"github.com/boz/ephemerald/builtin/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisPool(t *testing.T) {
	pool, err := redis.DefaultBuilder().
		WithSize(1).
		Create()

	require.NoError(t, err)

	defer func() {
		require.NoError(t, pool.Stop())
	}()

	require.NoError(t, pool.WaitReady())

	item, err := pool.Checkout()
	require.NoError(t, err)
	defer pool.Return(item)

	db, err := rredis.DialURL(item.URL)

	require.NoError(t, err)

	_, err = db.Do("PING")
	assert.NoError(t, err)

	pool.Return(item)
}

func TestMain(m *testing.M) {
	logrus.SetLevel(logrus.DebugLevel)
	m.Run()
}
