package events

import (
	"fmt"
	"sync"
)

type InMemoryEventStore struct {
	streams     map[string][]Event
	subscribers map[string][]EventHandler
	mutex       sync.RWMutex
	position    int
	allEvents   []Event
}

func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		streams:     make(map[string][]Event),
		subscribers: make(map[string][]EventHandler),
		allEvents:   make([]Event, 0),
	}
}

func (s *InMemoryEventStore) AppendEvent(streamID string, event Event) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.streams[streamID] == nil {
		s.streams[streamID] = make([]Event, 0)
	}

	eventWithVersion := BaseEvent{
		EventType:    event.Type(),
		Stream:       streamID,
		EventData:    event.Data(),
		EventTime:    event.Timestamp(),
		EventVersion: len(s.streams[streamID]) + 1,
	}

	s.streams[streamID] = append(s.streams[streamID], eventWithVersion)
	s.allEvents = append(s.allEvents, eventWithVersion)
	s.position++

	go s.notifySubscribers(eventWithVersion)

	return nil
}

func (s *InMemoryEventStore) ReadEvents(streamID string, fromVersion int) ([]Event, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	events, exists := s.streams[streamID]
	if !exists {
		return []Event{}, nil
	}

	if fromVersion < 1 {
		fromVersion = 1
	}

	if fromVersion > len(events) {
		return []Event{}, nil
	}

	return events[fromVersion-1:], nil
}

func (s *InMemoryEventStore) ReadAllEvents(fromPosition int) ([]Event, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if fromPosition < 0 {
		fromPosition = 0
	}

	if fromPosition >= len(s.allEvents) {
		return []Event{}, nil
	}

	return s.allEvents[fromPosition:], nil
}

func (s *InMemoryEventStore) Subscribe(eventTypes []string, handler EventHandler) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for _, eventType := range eventTypes {
		if s.subscribers[eventType] == nil {
			s.subscribers[eventType] = make([]EventHandler, 0)
		}
		s.subscribers[eventType] = append(s.subscribers[eventType], handler)
	}

	return nil
}

func (s *InMemoryEventStore) Unsubscribe(handler EventHandler) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for eventType, handlers := range s.subscribers {
		newHandlers := make([]EventHandler, 0)
		for _, h := range handlers {
			if h != handler {
				newHandlers = append(newHandlers, h)
			}
		}
		s.subscribers[eventType] = newHandlers
	}

	return nil
}

func (s *InMemoryEventStore) notifySubscribers(event Event) {
	s.mutex.RLock()
	handlers := s.subscribers[event.Type()]
	s.mutex.RUnlock()

	for _, handler := range handlers {
		if handler.CanHandle(event.Type()) {
			go func(h EventHandler, e Event) {
				if err := h.Handle(e); err != nil {
					fmt.Printf("Error handling event %s: %v\n", e.Type(), err)
				}
			}(handler, event)
		}
	}
}
