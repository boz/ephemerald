package types

import (
	"math/rand"
	"strconv"
)

type ID string

type Instance struct {
	ID     ID
	PoolID ID
	Host   string
	Port   int
}

func NewID() (ID, error) {
	id := rand.Uint64()
	return ID(strconv.FormatUint(id, 8)), nil
}
