package pg_test

import (
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
	"github.com/ovrclk/cpool/builtin/pg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPGPool(t *testing.T) {
	pool, err := pg.DefaultBuilder().
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

	db, err := sql.Open("postgres", item.URL())

	require.NoError(t, err)

	_, err = db.Query("SELECT 1")
	assert.NoError(t, err)

	pool.Return(item)
}

func TestMain(m *testing.M) {
	//logrus.SetLevel(logrus.DebugLevel)
	m.Run()
}
