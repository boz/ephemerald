package types

import "github.com/docker/docker/api/types/events"

type EventType string

const (
	EventTypeDocker   = "docker"
	EventTypePool     = "pool"
	EventTypeInstance = "instance"
)

type EventAction string

const (
	EventActionCreate     EventAction = "creating"
	EventActionStart                  = "starting"
	EventActionCheck                  = "checking"
	EventActionInitialize             = "initializing"
	EventActionReady                  = "ready"
	EventActionCheckout               = "checkout"
	EventActionReset                  = "resetting"
	EventActionKill                   = "killing"
	EventActionDone                   = "done"
)

var _ BusEvent = Event{}

type BusEvent interface {
	GetType() EventType
	GetAction() EventAction
	GetPool() ID
	GetInstance() ID
}

type Event struct {
	Type     EventType
	Action   EventAction
	Pool     ID
	Instance ID
}

func (ev Event) GetType() EventType {
	return ev.Type
}

func (ev Event) GetAction() EventAction {
	return ev.Action
}

func (ev Event) GetPool() ID {
	return ev.Pool
}

func (ev Event) GetInstance() ID {
	return ev.Instance
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

func (ev DockerEvent) GetPool() ID {
	return ev.Pool
}

func (ev DockerEvent) GetInstance() ID {
	return ev.Instance
}
