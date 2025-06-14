package events

import (
	"time"
)

type Event interface {
	Type() string
	StreamID() string
	Data() interface{}
	Timestamp() time.Time
	Version() int
}

type EventHandler interface {
	Handle(event Event) error
	CanHandle(eventType string) bool
}

type EventStore interface {
	AppendEvent(streamID string, event Event) error
	ReadEvents(streamID string, fromVersion int) ([]Event, error)
	ReadAllEvents(fromPosition int) ([]Event, error)
	Subscribe(eventTypes []string, handler EventHandler) error
	Unsubscribe(handler EventHandler) error
}

type BaseEvent struct {
	EventType    string
	Stream       string
	EventData    interface{}
	EventTime    time.Time
	EventVersion int
}

func (e BaseEvent) Type() string {
	return e.EventType
}

func (e BaseEvent) StreamID() string {
	return e.Stream
}

func (e BaseEvent) Data() interface{} {
	return e.EventData
}

func (e BaseEvent) Timestamp() time.Time {
	return e.EventTime
}

func (e BaseEvent) Version() int {
	return e.EventVersion
}

func NewEvent(eventType, streamID string, data interface{}) Event {
	return BaseEvent{
		EventType:    eventType,
		Stream:       streamID,
		EventData:    data,
		EventTime:    time.Now(),
		EventVersion: 1,
	}
}
