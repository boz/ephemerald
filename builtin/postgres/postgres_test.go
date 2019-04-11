package postgres_test

// func TestActionsPingExec(t *testing.T) {
// 	files := []string{"pool.json", "pool.yaml"}

// 	for _, file := range files {
// 		testutil.RunPoolFromFile(t, file, func(p params.Params) {
// 			db, err := sql.Open("postgres", p.Url)
// 			require.NoError(t, err, file)
// 			defer db.Close()

// 			rows, err := db.Query("SELECT COUNT(*) FROM users")
// 			require.NoError(t, err, file)
// 			defer rows.Close()

// 			require.True(t, rows.Next(), file)

// 			var count int
// 			require.NoError(t, rows.Scan(&count), file)
// 			require.Equal(t, 0, count, file)
// 		})
// 	}
// }

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
