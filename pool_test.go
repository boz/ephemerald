package ephemerald

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPool(t *testing.T) {
	config := NewConfig().
		WithImage("postgres").
		ExposePort("tcp", 5432).
		WithLabel("test", t.Name())

	prov := BuildProvisioner().
		WithInitialize(func(_ context.Context, si StatusItem) error {
			return nil
		}).
		WithReset(func(_ context.Context, si StatusItem) error {
			return nil
		}).Create()

	pool, err := NewPool(config, 1, prov)
	require.NoError(t, err)

	defer func() {
		assert.NoError(t, pool.Stop())
	}()

	require.NoError(t, pool.WaitReady())

	item, err := pool.Checkout()
	require.NoError(t, err)

	assert.NotNil(t, item)
	pool.Return(item)
}
