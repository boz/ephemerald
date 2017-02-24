package vault_test

import (
	"context"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/ovrclk/cpool/builtin/vault"
	"github.com/stretchr/testify/require"
)

func TestRedisPool(t *testing.T) {
	ctx := context.Background()

	pool, err := vault.DefaultBuilder().
		WithSize(1).
		Create()

	require.NoError(t, err)

	require.NoError(t, pool.WaitReady())

	defer func() {
		require.NoError(t, pool.Stop())
	}()

	item, err := pool.Checkout()
	require.NoError(t, err)
	defer pool.Return(item)

	err = vault.Ping(ctx, item.URL())
	require.NoError(t, err)

	pool.Return(item)
}

func TestMain(m *testing.M) {
	logrus.SetLevel(logrus.DebugLevel)
	m.Run()
}
