package eventsourcing

import "reflect"

// The standardEventRegistry is the default implementation of EventRegistry that stores
// event information for an aggregate in an internally managed structure.
type standardEventRegistry struct {
	events map[EventType]reflect.Type
}

// NewStandardEventRegistry creates an instance of a plain EventRegistry that
// stores information about event types in an internal map.
func NewStandardEventRegistry() EventRegistry {
	return &standardEventRegistry{
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

// RegisterEvent registers an event type with the registry
func (reg standardEventRegistry) RegisterEvent(event Event) EventType {
	eventTypeValue := reflect.TypeOf(event)
	eventTypeName := eventTypeValue.String()
	eventType := EventType(eventTypeName)
	reg.events[eventType] = eventTypeValue
	return eventType
}

// GetEventType determines the event type label for a given event instance.
func (reg standardEventRegistry) GetEventType(event interface{}) (EventType, bool) {
	eventTypeValue := reflect.TypeOf(event)
	eventTypeName := eventTypeValue.String()
	eventType := EventType(eventTypeName)
	_, found := reg.events[eventType]
	return eventType, found
}
