package postgres_test

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/boz/ephemerald/testutil"
	"github.com/boz/ephemerald/types"
	"github.com/stretchr/testify/require"
)

func TestActionsPingExec(t *testing.T) {
	t.SkipNow()

	files := []string{"_testdata/pool.json", "_testdata/pool.yaml"}

	for _, file := range files {
		testutil.WithCheckoutFromFile(t, file, func(co *types.Checkout) {

			url := fmt.Sprintf("postgres://%v:%v@%v:%v/%v?sslmode=disable",
				co.Vars["username"],
				co.Vars["password"],
				co.Host,
				co.Port,
				co.Vars["database"])

			db, err := sql.Open("postgres", url)
			require.NoError(t, err, file)
			defer db.Close()

			rows, err := db.Query("SELECT COUNT(*) FROM users")
			require.NoError(t, err, file)
			defer rows.Close()

			require.True(t, rows.Next(), file)

			var count int
			require.NoError(t, rows.Scan(&count), file)
			require.Equal(t, 0, count, file)
		})
	}
}

// func TestActionTruncate(t *testing.T) {
// 	testutil.WithPoolFromFile(t, "pool.json", func(pool ephemerald.Pool) {
// 		username := "testuser"

// 		func() {
// 			p, err := pool.Checkout()
// 			require.NoError(t, err)
// 			defer pool.Return(p)

// 			db, err := sql.Open("postgres", p.Url)
// 			require.NoError(t, err)
// 			defer db.Close()

// 			_, err = db.Exec("INSERT INTO users (name) VALUES ($1)", username)
// 			require.NoError(t, err)
// 		}()

// 		func() {
// 			p, err := pool.Checkout()
// 			require.NoError(t, err)
// 			defer pool.Return(p)

// 			db, err := sql.Open("postgres", p.Url)
// 			require.NoError(t, err)
// 			defer db.Close()

// 			rows, err := db.Query("SELECT COUNT(*) FROM users WHERE name = $1", username)
// 			require.NoError(t, err)
// 			defer rows.Close()

// 			require.True(t, rows.Next())

// 			var count int
// 			require.NoError(t, rows.Scan(&count))
// 			require.Equal(t, 0, count)
// 		}()
// 	})
// }
