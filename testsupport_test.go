package eventsourcing

// This content is a code-clone of the utilities/test code, but without
// prefixes. This exists purely because we need some simple test model code
// for the core framework tests, but also we don't want this stuff to become
// part of the public 'core' package code. Duplicating this small volume of
// code was the least-bad way that sprung to mind.

var counterRegistry EventRegistry

func init() {
	counterRegistry = NewStandardEventRegistry("Testing")
	counterRegistry.RegisterEvent(InitializeEvent{})
	counterRegistry.RegisterEvent(IncrementEvent{})
}

// GetTestRegistry returns the test registry for the library.
func GetTestRegistry() EventRegistry {
	return counterRegistry
}

// SimpleAggregate is a simple aggregate that counts up or
// down.
type SimpleAggregate struct {
	AggregateBase
	CurrentCount int `json:"current_count"`
	TargetValue  int `json:"target_value"`
}

// Initialize the aggregate
func (agg *SimpleAggregate) Initialize(key string, registry EventRegistry, store EventStore) {
	agg.AggregateBase.Initialize(key, registry, store, func() interface{} { return agg })
	agg.AggregateBase.AutomaticWireup(agg)
}

// HandleInitializeCommand handles the initialization of the counter.
func (agg *SimpleAggregate) HandleInitializeCommand(command InitializeCommand) ([]Event, error) {
	return []Event{
		InitializeEvent{
			TargetValue: command.TargetValue,
		},
	}, nil
}

// HandleIncrementCommand handles incrementing a counter.
func (agg *SimpleAggregate) HandleIncrementCommand(command IncrementCommand) ([]Event, error) {
	return []Event{
		IncrementCommand{
			IncrementBy: command.IncrementBy,
		},
	}, nil
}

// ReplayInitializeEvent applies an InitializeEvent to the model.
func (agg *SimpleAggregate) ReplayInitializeEvent(event InitializeEvent) {
	agg.TargetValue = event.TargetValue
}

// ReplayIncrementEvent applies an IncrementEvent to the model.
func (agg *SimpleAggregate) ReplayIncrementEvent(event IncrementEvent) {
	agg.CurrentCount += event.IncrementBy
}

// ReplayEventWithInvalidReturnMapping has a return value, and should not
// be wired up.
func (agg *SimpleAggregate) ReplayEventWithInvalidReturnMapping(event EventWithInvalidReturnMapping) int {
	agg.CurrentCount += event.IncrementBy
	return agg.CurrentCount
}

// ReplayEventWithTooManyArgumentsMapping has an extra parameter, and should not trigger
// an increment event.
func (agg *SimpleAggregate) ReplayEventWithTooManyArgumentsMapping(event EventWithInvalidReturnMapping, extraParameter int) {
	agg.CurrentCount += event.IncrementBy
}

// InitializeCommand is a command to initialize the aggregate
type InitializeCommand struct {
	// TargetValue is the value the counter will count towards.
	TargetValue int `json:"target_value"`
}

// IncrementCommand is a command to increment the total.
type IncrementCommand struct {
	IncrementBy int `json:"increment_by"`
}

// InitializeEvent is an event that initializes the current state
// of an event.
type InitializeEvent struct {
	// TargetValue is the value the counter will count towards.
	TargetValue int `json:"target_value"`
}

// IncrementEvent represents an event that increments the model value
type IncrementEvent struct {
	IncrementBy int `json:"increment_by"`
}

// EventWithInvalidReturnMapping is an event that does not have a good
// mapping - it has a reutrn value.
type EventWithInvalidReturnMapping struct {
	IncrementBy int `json:"increment_by"`
}

// EventWithTooManyArgumentsMapping is an event that does not have a good
// mapping - it has too many arguments to the replay method.
type EventWithTooManyArgumentsMapping struct {
	IncrementBy int `json:"increment_by"`
}

// UnknownEventTypeExample is an event that is just made out of the ether, and is not
// supported by SimpleAggregate. What should happen here is that the sequence
// jumps even though it's not wired up.
type UnknownEventTypeExample struct {
}

// The NullStore is a simple store implementation that does nothing besides
// clear the pending events on commit. Basically it's an event black-hole,
// and should only be used for stateless tests.
type NullStore struct {
}

// Close the event store
func (store *NullStore) Close() error {
	return nil
}

// CommitEvents writes events to a backing store. However, for the NullStore we
// simply do nothing and discard anything sent in.
func (store *NullStore) CommitEvents(adapter StoreWriterAdapter) error {
	return nil
}

// Refresh the state of the aggregate from the store. Does nothing and will
// never change aggregate state.
func (store *NullStore) Refresh(adapter StoreLoaderAdapter) error {
	// Intentionally blank
	return nil
}

// NewNullStore creates a null-store, a store that does nothing but
// accept/discard any data.
func NewNullStore() EventStore {
	return &NullStore{}
}

// The errorStore is an event store that returns a defined error each time.
type errorStore struct {
	// errorToReturn is the error that gets returned each call
	errorToReturn error
}

// CreateErrorStore creates a store that fails every operation with an error.
func CreateErrorStore(err error) EventStore {
	return &errorStore{
		errorToReturn: err,
	}
}

// Close the store
func (store *errorStore) Close() error {
	return nil
}

// CommitEvents writes events to a backing store. However, for the errorStore we
// simply return an error.
func (store *errorStore) CommitEvents(adapter StoreWriterAdapter) error {
	return store.errorToReturn
}

// Refresh the state of the aggregate from the store. Returns an error.
func (store *errorStore) Refresh(adapter StoreLoaderAdapter) error {
	return store.errorToReturn
}
