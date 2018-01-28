package eventsourcing

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/mitchellh/mapstructure"
)

const (
	// HandleMethodPrefix is a prefix used for command handler methods
	HandleMethodPrefix = "Handle"

	// ReplayMethodPrefix is the prefix used for event replay methods
	ReplayMethodPrefix = "Replay"
)

// AggregateBase is an implementation of Aggregate that provides a lot of shared
// boilerplate code.
type AggregateBase struct {
	// key is a Unique per-aggregate key
	key string

	// committedSequenceNumber is the sequence number we have
	// committed up until.
	committedSequenceNumber int64

	// sequenceNumber contains the current revision number
	// of the aggregate (i.e. the number of events that have)
	// been applied in it's lifetime.
	sequenceNumber int64

	// commandHandlers is the set of command-handler functons known for this
	// instance.
	commandHandlers map[CommandType]CommandHandleFunc

	// eventReplay is a map of event replay functions
	eventReplay map[EventType]func(Event)

	// eventRegistry is the instance of EventRegistry that
	// defines our events.
	eventRegistry EventRegistry

	// eventStore is the instance of EventStore that is used
	// to persist and load our events
	eventStore EventStore

	// uncomittedEvents are events that have not been put into
	// a backing store yet.
	uncomittedEvents []Event

	// stateFunc is a function reference that loads the state of an object.
	// This is required because we generally only have a reference to the
	// nested AggregateBase and there's no way to get back to the parent.
	stateFunc StateFetchFunc
}

// Initialize sets the initial state of the AggregateBase and ensures we are
// in a suitable situation to start reasoning about the events that will happen.
func (agg *AggregateBase) Initialize(key string, registry EventRegistry, store EventStore, state StateFetchFunc) {
	agg.key = key
	agg.sequenceNumber = 0
	agg.committedSequenceNumber = 0
	agg.eventRegistry = registry
	agg.eventReplay = make(map[EventType]func(Event))
	agg.commandHandlers = make(map[CommandType]CommandHandleFunc)
	agg.eventStore = store
	agg.uncomittedEvents = make([]Event, 0)
	agg.stateFunc = state
}

// Handle processes a command against the aggregate.
func (agg *AggregateBase) Handle(command Command) error {
	return agg.Run(func() error {
		return agg.handleInternal(command)
	})
}

// handleInternal determines the replay method to use, and then dispatches it.
func (agg *AggregateBase) handleInternal(command Command) error {
	commandType := CommandType(reflect.TypeOf(command).String())

	handler, found := agg.commandHandlers[commandType]
	if !found {
		return fmt.Errorf("Unsupported command type: %v", commandType)
	}

	events, errResult := handler(command)
	if errResult != nil {
		return errResult
	}

	for _, evt := range events {
		agg.ApplyEvent(evt)
	}

	return nil
}

// Run performs a load, mutate, commit cycle on an aggregate
func (agg *AggregateBase) Run(callback func() error) error {
	// Load the current state of the aggregate
	errLoad := agg.Refresh()
	if errLoad != nil {
		return errLoad
	}

	// Mutate the model with the callback
	errMutate := callback()
	if errMutate != nil {
		return errMutate
	}

	// Commit
	errCommit := agg.Commit()
	if errCommit != nil {
		return errCommit
	}

	return nil
}

// AutomaticWireup performs automatic detection of event replay methods, looking
// for applyEventName methods on the current type.
func (agg *AggregateBase) AutomaticWireup(subject interface{}) {
	agg.commandHandlers = buildHandleMappings(subject)
	agg.eventReplay = buildReplayMappings(subject)
}

// ApplyEvent applies an event that has occurred to the aggregate base
// instance to mutate its state. Events that are not recognized are
// ignored, and all event application should be fail-safe.
func (agg *AggregateBase) ApplyEvent(event Event) {
	agg.applyEventInternal(event)
	agg.uncomittedEvents = append(agg.uncomittedEvents, event)
}

// applyEventInternal applies an event internally
func (agg *AggregateBase) applyEventInternal(event Event) {
	defer func() {
		agg.sequenceNumber++
	}()

	// Determine the event type
	eventType, found := agg.eventRegistry.GetEventType(event)
	if !found {
		// The event was not found, so assume that this
		// instance doesn't care about it. We simply bump
		// the sequence to acknowledge we've seen it.
		return
	}

	// Find the replay function
	replayFunction, found := agg.eventReplay[eventType]
	if !found {
		// The event is known, but no replay function is defined.
		return
	}

	// Replay the event
	replayFunction(event)
}

// DefineReplayMethod defines a method that replays events of a given event type.
func (agg *AggregateBase) DefineReplayMethod(eventType EventType, replay func(Event)) {
	agg.eventReplay[eventType] = replay
}

// Refresh reloads the current state of the aggregate from the underlying store.
func (agg *AggregateBase) Refresh() error {
	adapter := &aggregateBaseLoaderAdapter{
		aggregate: agg,
		state:     agg.stateFunc(),
	}

	return agg.eventStore.Refresh(adapter)
}

// GetKey fetches the key of this aggregate instance.
func (agg *AggregateBase) GetKey() string {
	return agg.key
}

// SequenceNumber gets the current sequence number of the aggregate.
func (agg *AggregateBase) SequenceNumber() int64 {
	return agg.sequenceNumber
}

// Commit commits the state of the aggregate, marking all events
// as having been accepted by a backing store. This does not itself
// cause persistence to occur.
func (agg *AggregateBase) Commit() error {
	// Store the events
	err := agg.eventStore.CommitEvents(&aggregateBaseStoreAdapter{
		aggregate: agg,
		state:     agg.stateFunc(),
	})
	if err != nil {
		return err
	}

	// Clear the uncomittedEvents array
	agg.uncomittedEvents = make([]Event, 0)
	agg.committedSequenceNumber = agg.sequenceNumber
	return nil
}

// getEventRegistry fetches the event registry associated with this
// aggregate instance.
func (agg *AggregateBase) getEventRegistry() EventRegistry {
	return agg.eventRegistry
}

// isDirty returns true if the aggregate has uncommitted events, false otherwise.
func (agg *AggregateBase) isDirty() bool {
	return len(agg.uncomittedEvents) > 0
}

// buildHandleMappings builds a set of command handler mappings for a type that has
// methods of a suitable interface. This allows wireup-by-convention for the base
// aggregate type.
func buildHandleMappings(subject interface{}) map[CommandType]CommandHandleFunc {
	commandHandlers := make(map[CommandType]CommandHandleFunc)
	subjectType := reflect.TypeOf(subject)
	totalMethods := subjectType.NumMethod()
	for methodIndex := 0; methodIndex < totalMethods; methodIndex++ {
		candidate := subjectType.Method(methodIndex)

		// Skip methods without prefix
		if !strings.HasPrefix(candidate.Name, HandleMethodPrefix) {
			continue
		}

		// Method should have one argument (+ an instance argument), two outputs
		if candidate.Type.NumIn() != 2 || candidate.Type.NumOut() != 2 {
			continue
		}

		handler := func(command Command) ([]Event, error) {
			result := candidate.Func.Call([]reflect.Value{
				reflect.ValueOf(subject),
				reflect.ValueOf(command),
			})

			if len(result) != 2 {
				return nil, fmt.Errorf("Too many return arguments, expected 2, got %v", len(result))
			}

			var events []Event
			if !result[0].IsNil() {
				events = result[0].Interface().([]Event)
			}

			var err error
			if !result[1].IsNil() {
				err = result[1].Interface().(error)
			}

			return events, err
		}

		// The event type is the second parameter in an instance
		// method, since the first parameter is the instance
		commandType := candidate.Type.In(1)
		commandTypeName := CommandType(commandType.String())
		commandHandlers[commandTypeName] = handler
	}
	return commandHandlers
}

// buildReplayMappings builds a set of event replay mappings for a type that has
// methods of a suitable interface. This allows wireup-by-convention for the base
// aggregate type.
func buildReplayMappings(subject interface{}) map[EventType]func(Event) {
	eventReplay := make(map[EventType]func(Event))
	subjectType := reflect.TypeOf(subject)
	totalMethods := subjectType.NumMethod()
	for methodIndex := 0; methodIndex < totalMethods; methodIndex++ {
		candidate := subjectType.Method(methodIndex)

		// Skip methods without prefix
		if !strings.HasPrefix(candidate.Name, ReplayMethodPrefix) {
			continue
		}

		// Method should have two arguments, no outputs
		if candidate.Type.NumIn() != 2 || candidate.Type.NumOut() != 0 {
			continue
		}

		handler := func(event Event) {
			candidate.Func.Call([]reflect.Value{
				reflect.ValueOf(subject),
				reflect.ValueOf(event),
			})
		}

		// The event type is the second parameter in an instance
		// method, since the first parameter is the instance
		eventType := candidate.Type.In(1)
		eventTypeName := EventType(eventType.String())
		eventReplay[eventTypeName] = handler
	}
	return eventReplay
}

// aggregateBaseLoaderAdapter is a loader adapter for derrivatives of
// AggregateBase
type aggregateBaseLoaderAdapter struct {
	aggregate *AggregateBase
	state     interface{}
}

// GetKey fetches the aggregate key
func (adapter *aggregateBaseLoaderAdapter) GetKey() string {
	return adapter.aggregate.GetKey()
}

// GetEventRegistry gets the event registry for this aggregate
func (adapter *aggregateBaseLoaderAdapter) GetEventRegistry() EventRegistry {
	return adapter.aggregate.getEventRegistry()
}

// SequenceNumber gets the current sequence number of the aggregate
func (adapter *aggregateBaseLoaderAdapter) SequenceNumber() int64 {
	return adapter.aggregate.SequenceNumber()
}

// IsDirty returns true if the aggregate is dirty/has uncommitted events
func (adapter *aggregateBaseLoaderAdapter) IsDirty() bool {
	return adapter.aggregate.isDirty()
}

// ReplayEvent replays an event that has already been persisted
func (adapter *aggregateBaseLoaderAdapter) ReplayEvent(event Event) {
	adapter.aggregate.applyEventInternal(event)
	adapter.aggregate.committedSequenceNumber++
}

// RestoreSnapshot sets the current position and restores the snapshot
// state over the top of the aggregate.
func (adapter *aggregateBaseLoaderAdapter) RestoreSnapshot(sequence int64, snapshot interface{}) error {
	config := &mapstructure.DecoderConfig{
		TagName:          "json",
		Result:           adapter.state,
		WeaklyTypedInput: true,
	}
	decoder, errDecoder := mapstructure.NewDecoder(config)
	if errDecoder != nil {
		return errDecoder
	}

	errDecode := decoder.Decode(snapshot)
	if errDecode == nil {
		adapter.aggregate.sequenceNumber = sequence
		adapter.aggregate.committedSequenceNumber = sequence
	}

	return errDecode
}

// aggregateBaseStoreAdapter is an event-store adapter for saving events
type aggregateBaseStoreAdapter struct {
	state     interface{}    // State is the untyped instance-level reference
	aggregate *AggregateBase // the nested AggregateBase within the state
}

// GetKey gets the key of the aggregate
func (adapter *aggregateBaseStoreAdapter) GetKey() string {
	return adapter.aggregate.GetKey()
}

// SequenceNumber fetches the current sequence numer of the aggregate.
func (adapter *aggregateBaseStoreAdapter) SequenceNumber() int64 {
	return adapter.aggregate.SequenceNumber()
}

// IsDirty checks if the aggregate beign stored is dirty
func (adapter *aggregateBaseStoreAdapter) IsDirty() bool {
	return adapter.aggregate.isDirty()
}

// GetEventRegistry fetches the event registry for this aggregate
func (adapter *aggregateBaseStoreAdapter) GetEventRegistry() EventRegistry {
	return adapter.aggregate.getEventRegistry()
}

// GetUncomittedEvents fetches the uncommitted events of this aggregate
func (adapter *aggregateBaseStoreAdapter) GetUncomittedEvents() (int64, []Event) {
	return adapter.aggregate.committedSequenceNumber, adapter.aggregate.uncomittedEvents
}

// GetState returns the aggregate state for serialization.
func (adapter *aggregateBaseStoreAdapter) GetState() interface{} {
	return adapter.state
}
