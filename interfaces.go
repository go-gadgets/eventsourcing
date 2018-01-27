package eventsourcing

// Aggregate is the interface for an event-sourced aggregate root.
// All common behaviours of an aggregate expected by the runtime are
// defined here.
type Aggregate interface {
	// Initialize sets up the initial state of the aggregate.
	Initialize(key string, registry EventRegistry, store EventStore)

	// ApplyEvent applies an event that has occurred to the aggregate
	// instance to mutate its state. Events that are not recognized are
	// ignored, and all event application is fail-safe.
	ApplyEvent(interface{})

	// Commit commits the state of the aggregate, persisting any
	// new events to the store.
	Commit() error

	// Refresh recovers the state of the aggregate from the underlying
	// store.
	Refresh() error

	// GetState gets the state of an aggregate
	GetState() interface{}
}

// EventDefinition defines the structure of an event.
type EventDefinition struct {
	// Detector is a function that determines if a specific runtime event
	// matches this event revisions type.
	Detector EventDetector

	// Factory method to create an instance of the event for this specific version.
	Factory EventFactory
}

// An EventDetector is a function that determines if the streamed
// event is an instance of the specified event revision. True indicates
// a match, false indicates a mis-match.
type EventDetector func(interface{}) bool

// EventFactory is a function that creates an event instance of a
// given type, ready to work with.
type EventFactory func() interface{}

// EventType is a string alias that represents the type of an event.
type EventType string

// EventRegistry defines a per-aggregate type registry of the events that are
// known to a specific aggregate.
type EventRegistry interface {
	// CreateEvent creates an instance of an event
	CreateEvent(EventType) interface{}

	// GetEventType determines the EventType of an event
	GetEventType(interface{}) (EventType, bool)

	// RegisterEvent registers an event
	RegisterEvent(interface{}) EventType
}

// EventStore defines the behaviours of a store that can load/save event streams
// for an aggregate.
type EventStore interface {
	// CommitEvents stores any events for the specified aggregate that are uncommitted
	// at this point in time.
	CommitEvents(writer StoreWriterAdapter) error

	// Refresh refreshes the state of the specified aggregate from the underlying store
	Refresh(reader StoreLoaderAdapter) error
}

// StateFetchFunc is a function that returns the state-value.
type StateFetchFunc func() interface{}
