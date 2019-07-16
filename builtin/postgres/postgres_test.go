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
	files := []string{"_testdata/pool.json", "_testdata/pool.yaml"}

	for _, file := range files {
		testutil.WithCheckoutFromFile(t, file, func(co *types.Checkout) {
			url := fmt.Sprintf("postgres://%v:%v@%v:%v/%v?sslmode=disable",
				"postgres",
				"",
				co.Host,
				co.Port,
				"postgres")

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
