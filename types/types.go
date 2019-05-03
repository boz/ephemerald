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
	ID         ID        `json:"id"`
	Name       string    `json:"name"`
	State      PoolState `json:"state"`
	Size       int       `json:"size"`
	Containers struct {
		Total    int `json:"total"`
		Ready    int `json:"ready"`
		Checkout int `json:"checkout"`
		Requests int `json:"requests"`
	} `json:"containers"`
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
	ID        ID            `json:"id"`
	PoolID    ID            `json:"pool-id"`
	State     InstanceState `json:"state"`
	Resets    int           `json:"resets,omitempty"`
	MaxResets int           `json:"max-resets,omitempty"`
	Host      string        `json:"host,omitempty"`
	Port      int           `json:"port,omitempty"`
}

type Checkout struct {
	InstanceID ID                `json:"instance-id"`
	PoolID     ID                `json:"pool-id"`
	Host       string            `json:"host"`
	Port       int               `json:"port"`
	Vars       map[string]string `json:"vars"`
}

type LifecycleActionState string

const (
	LifecycleActionStateRunning   LifecycleActionState = "running"
	LifecycleActionStateRetryWait                      = "retry-wait"
	LifecycleActionStateDone                           = "done"
)

type LifecycleAction struct {
	Name       string               `json:"name"`
	Type       string               `json:"type"`
	State      LifecycleActionState `json:"state"`
	Retries    uint                 `json:"retries"`
	MaxRetries uint                 `json:"max-retries"`

	Instance Instance `json:"instance"`

	Vars map[string]string `json:"vars,omitempty"`
}

func NewID() (ID, error) {
	id := rand.Uint64()
	return ID(strconv.FormatUint(id, 16)), nil
}
