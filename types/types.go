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
	ID    ID
	Name  string
	State PoolState
	Size  int
	Stats struct {
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
	Host      string
	Port      string
	Resets    int
	MaxResets int
}

type LifecycleActionState string

const (
	LifecycleActionStateRunning   LifecycleActionState = "running"
	LifecycleActionStateRetryWait                      = "retry-wait"
	LifecycleActionStateDone                           = "done"
)

type LifecycleAction struct {
	PoolID     ID
	InstanceID ID
	Name       string
	Type       string
	State      LifecycleActionState
	Retries    uint
	MaxRetries uint
}

type Checkout struct {
	PoolID     ID                `json:"pool-id"`
	InstanceID ID                `json:"instance-id"`
	NumResets  int               `json:"num-resets"`
	Host       string            `json:"host"`
	Port       string            `json:"port"`
	Params     map[string]string `json:"params"`
}

func NewID() (ID, error) {
	id := rand.Uint64()
	return ID(strconv.FormatUint(id, 16)), nil
}
