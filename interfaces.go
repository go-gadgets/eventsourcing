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
	ApplyEvent(Event)

	// Commit commits the state of the aggregate, persisting any
	// new events to the store.
	Commit() error

	// Refresh recovers the state of the aggregate from the underlying
	// store.
	Refresh() error

	// GetState gets the state of an aggregate
	GetState() interface{}
}

// Command is an interface that describes commands common attributes
type Command interface {
}

// CommandHandleFunc is a function that handles a command directly.
type CommandHandleFunc func(command Command) ([]Event, error)

// CommandHandler is an interface that describes the operations available on
// an instance that can follow the command-handler pattern.
type CommandHandler interface {
	// Handle a command, returning the resultant events (or an error)
	Handle(command Command) ([]Event, error)
}

// CommandType is a string-alias that represents a commands type, which
// can be used in maps.
type CommandType string

// Event is an interface that describes common attributes of events.
type Event interface {
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
type EventFactory func() Event

// EventType is a string alias that represents the type of an event.
type EventType string

// EventRegistry defines a per-aggregate type registry of the events that are
// known to a specific aggregate.
type EventRegistry interface {
	// CreateEvent creates an instance of an event
	CreateEvent(EventType) Event

	// Domain this registry contains events for
	Domain() string

	// GetEventType determines the EventType of an event
	GetEventType(interface{}) (EventType, bool)

	// RegisterEvent registers an event
	RegisterEvent(Event) EventType
}

// EventStore defines the behaviours of a store that can load/save event streams
// for an aggregate.
type EventStore interface {
	// CommitEvents stores any events for the specified aggregate that are uncommitted
	// at this point in time.
	CommitEvents(writer StoreWriterAdapter) error

	// Refresh refreshes the state of the specified aggregate from the underlying store
	Refresh(reader StoreLoaderAdapter) error

	// Close shuts down the storage driver.
	Close() error
}

// EventStoreWithMiddleware is an interface that describes an event-store with middleware
// support.
type EventStoreWithMiddleware interface {
	EventStore

	// Use a middleware
	Use(commit CommitMiddleware, refresh RefreshMiddleware, cleanup func() error)

	// HandleCleanup registers a cleanup/shutdown handler
	HandleCleanup(cleanup func() error)

	// HandleCommit registers middleware to handle commits
	HandleCommit(middleware CommitMiddleware)

	// HandleRefresh registers middleware to handle refreshes
	HandleRefresh(middleware RefreshMiddleware)
}

// EventConsumer is an interface that describes a consumer that allows multiple
// handlers to be attached, allowing events to be multiplexed to the handlers
// without needing to consume the same stream multiple times.
type EventConsumer interface {
	// Start consuming
	Start() error

	// Stop consuming
	Stop() error

	// AddHandler adds a handler to the set of handlers for this consumer.
	AddHandler(handler EventHandler)
}

// EventHandler is an interface that handles events that have been delivered from
// a publishing source
type EventHandler interface {
	// Handle the specified event and apply any consequences.
	Handle(event PublishedEvent) error
}

// EventPublisher is an interface that describes an event publisher sink that
// allows events to be distributed to other components.
type EventPublisher interface {
	// Publish an event. When the method returns the event should be committed/guaranteed
	// to have been distributed.
	Publish(key string, sequence int64, event Event) error
}

// PublishedEvent is a record of an event that's published to a queue or sink
type PublishedEvent struct {
	Domain   string      `json:"domain"`     // Domain the event belong sto
	Type     EventType   `json:"event_type"` // EventType
	Key      string      `json:"key"`        // Event key
	Sequence int64       `json:"sequence"`   // Sequence number
	Data     interface{} `json:"data"`       // Data
}

// StateFetchFunc is a function that returns the state-value.
type StateFetchFunc func() interface{}
