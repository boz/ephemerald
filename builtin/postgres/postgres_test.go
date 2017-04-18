package postgres_test

import (
	"database/sql"
	"testing"

	"github.com/boz/ephemerald"
	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/testutil"
	"github.com/stretchr/testify/require"
)

func TestActionsPingExec(t *testing.T) {
	testutil.RunPoolFromFile(t, "pool.json", func(p params.Params) {
		db, err := sql.Open("postgres", p.Url)
		require.NoError(t, err)
		defer db.Close()

		rows, err := db.Query("SELECT COUNT(*) FROM users")
		require.NoError(t, err)
		defer rows.Close()

		require.True(t, rows.Next())

		var count int
		require.NoError(t, rows.Scan(&count))
		require.Equal(t, 0, count)
	})
}

func TestActionTruncate(t *testing.T) {
	testutil.WithPoolFromFile(t, "pool.json", func(pool ephemerald.Pool) {
		username := "testuser"

		func() {
			p, err := pool.Checkout()
			require.NoError(t, err)
			defer pool.Return(p)

			db, err := sql.Open("postgres", p.Url)
			require.NoError(t, err)
			defer db.Close()

			_, err = db.Exec("INSERT INTO users (name) VALUES ($1)", username)
			require.NoError(t, err)
		}()

		func() {
			p, err := pool.Checkout()
			require.NoError(t, err)
			defer pool.Return(p)

			db, err := sql.Open("postgres", p.Url)
			require.NoError(t, err)
			defer db.Close()

			rows, err := db.Query("SELECT COUNT(*) FROM users WHERE name = $1", username)
			require.NoError(t, err)
			defer rows.Close()

			require.True(t, rows.Next())

			var count int
			require.NoError(t, rows.Scan(&count))
			require.Equal(t, 0, count)
		}()
	})
}
