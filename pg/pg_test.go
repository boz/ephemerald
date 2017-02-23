package pg_test

import (
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
	"github.com/ovrclk/cpool/pg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPGPool(t *testing.T) {
	pool, err := pg.DefaultBuilder().
		WithSize(1).
		Create()

	require.NoError(t, err)

	defer func() {
		require.NoError(t, pool.Stop())
	}()

	item := pool.Checkout()
	defer pool.Return(item)

	db, err := sql.Open("postgres", item.URL())

	require.NoError(t, err)

	_, err = db.Query("SELECT 1")
	assert.NoError(t, err)

	pool.Return(item)
}
