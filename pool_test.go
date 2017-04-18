package ephemerald_test

import (
	"testing"

	"github.com/boz/ephemerald/testutil"
)

func TestPool(t *testing.T) {
	testutil.RunPoolFromFile(t, "pool.redis.json", nil)
}
