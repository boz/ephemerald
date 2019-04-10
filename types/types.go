package types

import (
	"math/rand"
	"strconv"
)

type ID string

type Container struct {
	ID     ID
	PoolID ID
}

func NewID() (ID, error) {
	id := rand.Uint64()
	return ID(strconv.FormatUint(id, 8)), nil
}
