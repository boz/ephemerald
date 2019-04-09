package types

import (
	"math/rand"
	"strconv"
)

type ID string
type PoolID = ID

type ContainerID struct {
	ID     ID
	PoolID PoolID
}

func NewID() (ID, error) {
	id := rand.Uint64()
	return ID(strconv.FormatUint(id, 8)), nil
}
