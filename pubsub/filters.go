package pubsub

import "github.com/boz/ephemerald/types"

type Filter func(types.BusEvent) bool

func FilterNone(_ types.BusEvent) bool {
	return true
}
