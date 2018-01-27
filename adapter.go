package eventsourcing

// Adapter is an interface that exposes state information about the aggregate
// being operated on.
type Adapter interface {
	// GetKey fetches the aggregate key
	GetKey() string
}

// AdapterPositional is an adapter which can introspect about
// where an aggregate is at in terms of it's history.
type AdapterPositional interface {
	Adapter

	// SequenceNumber fetches the current sequence number
	SequenceNumber() int64
}

// AdapterWithEvents is variant of Adapter that is required where components
// need to reason about event types in an abstract way (i.e. Event Reader adapters)
type AdapterWithEvents interface {
	AdapterPositional

	// GetEventRegistry gets the event registry to use
	GetEventRegistry() EventRegistry

	// IsDirty returns true if the aggregate has uncomitted state.
	IsDirty() bool
}

// StoreLoaderAdapter represents an adapter that can be used
// to modify an aggregate in response to a load/refresh operation
type StoreLoaderAdapter interface {
	AdapterWithEvents

	// ReplayEvent applies an event that has already been persisted
	ReplayEvent(event interface{})

	// RestoreSnapshot applies a snapshot state, if available
	RestoreSnapshot(sequence int64, state interface{}) error
}

// StoreWriterAdapter is an adapter interface that defines the inputs an aggregate
// gives to a store for writing/comitting new events.
type StoreWriterAdapter interface {
	AdapterWithEvents

	// GetUncomittedEvent gets the comitted sequence number, and any
	// events that have been added since hte last commit. This can been
	// used by a backing store to write data.
	GetUncomittedEvents() (int64, []interface{})

	// GetState returns the state of the aggregate in it's current
	// sequence/position, which may be required when snapshotting.
	GetState() interface{}
}

// SnapReaderAdapter represents an adapter that can be used to read snapshots
// from a storage provider.
type SnapReaderAdapter interface {
	Adapter

	// SetState apply the state state form the reader over the top of the
	// focused instance and set the current sequence number accordingly.
	SetState(state interface{}, snapSequence int64)
}

// SnapWriterAdapter isan adapter interface that defines the inputs and operations
// expected by a snapshot provider in order to store snapshots.
type SnapWriterAdapter interface {
	AdapterPositional

	// GetState returns the state of the aggregate for persistence.
	// The structure returned should be directly marshalled to JSON.
	ReadState() interface{}
}
