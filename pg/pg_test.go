package pg_test

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/ovrclk/cpool/pg"
	"github.com/stretchr/testify/require"
)

func TestPGPool(t *testing.T) {
	pool, err := pg.NewBuilder().
		WithSize(2).
		Create()
	require.NoError(t, err)

	defer func() {
		require.NoError(t, pool.Stop())
	}()

	item := pool.Checkout()

	db, err := sql.Open("postgres", item.URL())

	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		_, err = db.Query("SELECT 1")
		if err == nil {
			break
		}
		time.Sleep(5 * time.Second)
		t.Logf("retrying try %v", i)
	}

	require.NoError(t, err)

	pool.Return(item)

}
