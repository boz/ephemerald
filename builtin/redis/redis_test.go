package redis_test

import (
	"fmt"
	"testing"

	"github.com/boz/ephemerald/testutil"
	"github.com/boz/ephemerald/types"
	rredis "github.com/garyburd/redigo/redis"
	"github.com/stretchr/testify/require"
)

func TestActionExec(t *testing.T) {
	files := []string{"_testdata/pool.json", "_testdata/pool.yaml"}

	for _, file := range files {
		testutil.WithCheckoutFromFile(t, file, func(co *types.Checkout) {

			address := fmt.Sprintf("%v:%v", co.Host, co.Port)
			db := 0

			conn, err := rredis.Dial("tcp", address,
				rredis.DialDatabase(db))
			require.NoError(t, err)

			defer conn.Close()

			_, err = conn.Do("PING")
			require.NoError(t, err, file)
		})
	}
}
