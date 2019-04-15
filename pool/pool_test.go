package pool_test

import (
	"context"
	"testing"

	"github.com/boz/ephemerald/pool"
	"github.com/boz/ephemerald/testutil"
	"github.com/boz/ephemerald/types"
	"github.com/stretchr/testify/require"
)

func Test_Pool(t *testing.T) {
	testutil.WithPoolFromFile(t, "../_testdata/pools.json", func(pool pool.Pool) {
		ctx := context.Background()

		params, err := pool.Checkout(ctx)
		require.NoError(t, err)

		require.NoError(t, pool.Release(ctx, types.ID(params.ID)))
	})
}
