package types

import "github.com/docker/docker/api/types/events"

type EventType string

const (
	EventTypeDocker          EventType = "docker"
	EventTypePool                      = "pool"
	EventTypeInstance                  = "instance"
	EventTypeLifecycleAction           = "lifecycle-action"
)

type EventAction string

const (
	EventActionStart         EventAction = "start"
	EventActionEnterState                = "enter-state"
	EventActionUpdate                    = "update"
	EventActionDone                      = "done"
	EventActionAttemptFailed             = "attempt-failed"
)

type Status string

const (
	StatusInProgress = "in-progress"
	StatusSuccess    = "success"
	StatusFailure    = "failure"
)

var _ BusEvent = Event{}

type BusEvent interface {
	GetType() EventType
	GetAction() EventAction
	GetPoolID() ID
	GetInstanceID() ID
}

type Event struct {
	Type            EventType
	Action          EventAction
	Pool            *Pool
	Instance        *Instance
	LifecycleAction *LifecycleAction
	Status          Status
	Message         string
}

func (ev Event) GetType() EventType {
	return ev.Type
}

func (ev Event) GetAction() EventAction {
	return ev.Action
}

func (ev Event) GetPoolID() ID {
	switch {
	case ev.Pool != nil:
		return ev.Pool.ID
	case ev.Instance != nil:
		return ev.Instance.PoolID
	case ev.LifecycleAction != nil:
		return ev.LifecycleAction.PoolID
	default:
		return ID("")
	}
}

func (ev Event) GetInstanceID() ID {
	switch {
	case ev.Instance != nil:
		return ev.Instance.ID
	case ev.LifecycleAction != nil:
		return ev.LifecycleAction.InstanceID
	default:
		return ID("")
	}
}

type DockerEvent struct {
	Node     string
	Pool     ID
	Instance ID
	Message  events.Message
}

func (ev DockerEvent) GetType() EventType {
	return EventTypeDocker
}

func (ev DockerEvent) GetAction() EventAction {
	return EventAction(ev.Message.Action)
}

func (ev DockerEvent) GetPoolID() ID {
	return ev.Pool
}

func (ev DockerEvent) GetInstanceID() ID {
	return ev.Instance
}
