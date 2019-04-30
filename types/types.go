package types

import (
	"math/rand"
	"strconv"
)

type ID string

type PoolState string

const (
	PoolStateStart    PoolState = "starting"
	PoolStateResolve            = "image-pull"
	PoolStateRun                = "running"
	PoolStateShutdown           = "shutting-down"
	PoolStateDone               = "done"
)

type Pool struct {
	ID         ID
	Name       string
	State      PoolState
	Size       int
	Containers struct {
		Total    int
		Ready    int
		Checkout int
		Requests int
	}
}

type InstanceState string

const (
	InstanceStateCreate     InstanceState = "creating"
	InstanceStateStart                    = "starting"
	InstanceStateCheck                    = "checking"
	InstanceStateInitialize               = "initializing"
	InstanceStateReady                    = "ready"
	InstanceStateCheckout                 = "checkout"
	InstanceStateReset                    = "resetting"
	InstanceStateKill                     = "killing"
	InstanceStateDone                     = "done"
)

type Instance struct {
	ID        ID
	PoolID    ID
	State     InstanceState
	Resets    int
	MaxResets int
	Host      string
	Port      string
}

type Checkout struct {
	InstanceID ID                `json:"instance-id"`
	PoolID     ID                `json:"pool-id"`
	Host       string            `json:"host"`
	Port       string            `json:"port"`
	Vars       map[string]string `json:"vars"`
}

type LifecycleActionState string

const (
	LifecycleActionStateRunning   LifecycleActionState = "running"
	LifecycleActionStateRetryWait                      = "retry-wait"
	LifecycleActionStateDone                           = "done"
)

type LifecycleAction struct {
	Name       string
	Type       string
	State      LifecycleActionState
	Retries    uint
	MaxRetries uint

	Instance struct {
		ID        ID     `json:"id"`
		PoolID    ID     `json:"pool-id"`
		NumResets int    `json:"num-resets"`
		MaxResets int    `json:"max-resets"`
		Host      string `json:"host"`
		Port      string `json:"port"`
	}

	Vars map[string]string
}

func NewID() (ID, error) {
	id := rand.Uint64()
	return ID(strconv.FormatUint(id, 16)), nil
}
