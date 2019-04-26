package poolset_test

import (
	"testing"
	"time"

	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/poolset"
	"github.com/boz/ephemerald/scheduler"
	"github.com/boz/ephemerald/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Poolset(t *testing.T) {
	ctx := testutil.Context()
	node := testutil.Node(t, ctx)

	bus := testutil.Bus(t, ctx)
	defer func() {
		require.NoError(t, bus.Shutdown())
	}()
	sched := scheduler.New(ctx, bus, node)

	var (
		rcfg config.Pool
		pcfg config.Pool
	)

	testutil.ReadFile(t, "../_testdata/pool.redis.yml", &rcfg)
	testutil.ReadFile(t, "../_testdata/pool.postgres.yml", &pcfg)

	pset, err := poolset.New(ctx, bus, sched)
	require.NoError(t, err)
	defer func() {
		pset.Shutdown()
		<-pset.Done()
	}()

	rpool, err := pset.Create(ctx, rcfg)
	if !assert.NoError(t, err, "redis pool") {
		return
	}

	ppool, err := pset.Create(ctx, pcfg)
	if !assert.NoError(t, err, "postgres pool") {
		return
	}

	_, err = pset.Create(ctx, pcfg)
	if !assert.Error(t, err, "postgres second pool") {
		return
	}

	{
		pool, err := pset.Get(ctx, rpool.ID())
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Equal(t, rpool.ID(), pool.ID()) {
			return
		}
		_, err = pool.Checkout(ctx)
		if !assert.NoError(t, err) {
			return
		}
	}

	{
		pool, err := pset.Get(ctx, ppool.ID())
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Equal(t, ppool.ID(), pool.ID()) {
			return
		}

		_, err = pool.Checkout(ctx)
		if !assert.NoError(t, err) {
			return
		}
	}

	{
		pools, err := pset.List(ctx)
		if !assert.NoError(t, err) {
			return
		}
		assert.Len(t, pools, 2)
		assert.True(t, pools[0].ID() == rpool.ID() || pools[0].ID() == ppool.ID())
		assert.True(t, pools[1].ID() == rpool.ID() || pools[1].ID() == ppool.ID())
	}

	assert.NoError(t, pset.Delete(ctx, rpool.ID()))
	select {
	case <-time.After(time.Second):
		assert.Fail(t, "delete")
	case <-rpool.Done():
	}

	{
		pools, err := pset.List(ctx)
		if !assert.NoError(t, err) {
			return
		}
		assert.Len(t, pools, 1)
		assert.True(t, pools[0].ID() == ppool.ID())
	}

	pset.Shutdown()

	select {
	case <-time.After(time.Second):
		assert.Fail(t, "ppool shutdown")
	case <-ppool.Done():
	}

	select {
	case <-time.After(time.Second):
		assert.Fail(t, "pset shutdown")
	case <-pset.Done():
	}

}
