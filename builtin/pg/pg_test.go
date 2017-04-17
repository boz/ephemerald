package pg_test

import (
	"database/sql"
	"testing"

	"github.com/boz/ephemerald/builtin/pg"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPGPool(t *testing.T) {
	pool, err := pg.DefaultBuilder().
		WithSize(1).
		WithLabel("test", "pg_test.TestPGPool").
		Create()

	require.NoError(t, err)

	require.NoError(t, pool.WaitReady())

	defer func() {
		assert.NoError(t, pool.Stop())
	}()

	item, err := pool.Checkout()
	require.NoError(t, err)
	defer pool.Return(item)

	db, err := sql.Open("postgres", item.URL)

	require.NoError(t, err)

	_, err = db.Query("SELECT 1")
	assert.NoError(t, err)
}
