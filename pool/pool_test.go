package pool_test

import (
	"testing"

	"github.com/boz/ephemerald/testutil"
)

func Test_Pool(t *testing.T) {
	testutil.RunPoolFromFile(t, "../_testdata/pool.redis.yml")
	testutil.RunPoolFromFile(t, "../_testdata/pool.postgres.yml")
}
