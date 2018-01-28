package eventsourcing

import (
	"reflect"
)

// The standardEventRegistry is the default implementation of EventRegistry that stores
// event information for an aggregate in an internally managed structure.
type standardEventRegistry struct {
	domain string                     // Name of the domain
	events map[EventType]reflect.Type // events to type mapping
}

// NewStandardEventRegistry creates an instance of a plain EventRegistry that
// stores information about event types in an internal map. The string parameter
// is the name of the domain/bounded-context in which our events live.
func NewStandardEventRegistry(domain string) EventRegistry {
	return &standardEventRegistry{
		domain: domain,
		events: make(map[EventType]reflect.Type),
	}
}

// CreateEvent creates a new instance of the specified event type.
func (reg standardEventRegistry) CreateEvent(eventType EventType) Event {
	// Look for the type in the known types map
	entry, exists := reg.events[eventType]

	// if no type exists, assume default polymorphic
	if !exists {
		return make(map[string]interface{})
	}

	newInstance := reflect.New(entry)

	return newInstance.Interface()
}

// Domain that this registry contains events for.
func (reg standardEventRegistry) Domain() string {
	return reg.domain
}

// RegisterEvent registers an event type with the registry
func (reg standardEventRegistry) RegisterEvent(event Event) EventType {
	eventTypeValue := reflect.TypeOf(event)
	eventTypeName := NormalizeEventName(eventTypeValue.String())
	eventType := EventType(eventTypeName)
	reg.events[eventType] = eventTypeValue
	return eventType
}

// GetEventType determines the event type label for a given event instance.
func (reg standardEventRegistry) GetEventType(event interface{}) (EventType, bool) {
	eventTypeValue := reflect.TypeOf(event)
	eventTypeName := NormalizeEventName(eventTypeValue.String())
	eventType := EventType(eventTypeName)
	_, found := reg.events[eventType]
	return eventType, found
}
