package types

import "github.com/docker/docker/api/types/events"

type EventType string

const (
	EventTypeDocker    = "docker"
	EventTypePool      = "pool"
	EventTypeContainer = "container"
)

type BusEvent interface {
	GetType() EventType
	GetPool() ID
	GetContainer() ID
}

type Event struct {
	Type      EventType
	Pool      ID
	Container ID
}

func (ev Event) GetPool() ID {
	return ev.Pool
}

func (ev Event) GetContainer() ID {
	return ev.Container
}

type DockerEvent struct {
	Node      string
	Pool      ID
	Container ID
	Message   events.Message
}

func (ev DockerEvent) GetType() EventType {
	return EventTypeDocker
}

func (ev DockerEvent) GetPool() ID {
	return ev.Pool
}

func (ev DockerEvent) GetContainer() ID {
	return ev.Container
}
