package keyvalue

import (
	"fmt"
	"reflect"

	"github.com/go-gadgets/eventsourcing"
	"github.com/mitchellh/mapstructure"
)

// Options is a structure containing the function callbacks
// required for a simple key-value store to be used as an event storage
// engine.
type Options struct {
	CheckSequence SequenceExistsCallback // Check function to see if seq exists
	FetchEvents   FetchCallback          // Fetch events function
	PutEvents     PutCallback            // Put events function
	Close         CloseCallback          // Close callback
}

// Event is a raw event within a key-value store.
type Event struct {
	EventType eventsourcing.EventType `json:"type"`
	EventData interface{}             `json:"data"`
}

// KeyedEvent is an event with a key and sequence.
type KeyedEvent struct {
	Key       string                  `json:"key"`
	Sequence  int64                   `json:"sequence"`
	EventType eventsourcing.EventType `json:"type"`
	EventData interface{}             `json:"data"`
}

// SequenceExistsCallback is a function that checks if  given offset exists
// already within the store.
type SequenceExistsCallback func(key string, seq int64) (bool, error)

// FetchCallback is a function that describes the behaviour for
// fetching events from a key-value store. Typically this is a sequential
// crawl forward from the specified sequence for a partitioning key.
type FetchCallback func(key string, seq int64) ([]KeyedEvent, error)

// PutCallback is a function that puts events into the store.
type PutCallback func(events []KeyedEvent) error

// CloseCallback closes the KVS
type CloseCallback func() error

// store is the type for the key-value backed storage provider.
type store struct {
	options Options // Functions for callbacks, other options.
}

// NewStore creates a new key-value store that provides a simple
// baseline set of event storage capabilities
func NewStore(options Options) eventsourcing.EventStore {
	return &store{
		options: options,
	}
}

// Close the event-store
func (store *store) Close() error {
	if store.options.Close != nil {
		return store.options.Close()
	}
	return nil
}

// CommitEvents writes new events for an aggregate to the storage provider.
func (store *store) CommitEvents(writer eventsourcing.StoreWriterAdapter) error {
	key := writer.GetKey()
	registry := writer.GetEventRegistry()
	currentSequenceNumber, events := writer.GetUncommittedEvents()

	// If we're writing beyond zero, we need to check that there's priors.
	if currentSequenceNumber > 0 {
		exists, errExists := store.options.CheckSequence(key, currentSequenceNumber)
		if errExists != nil {
			return errExists
		}

		if !exists {
			return fmt.Errorf(
				"StoreError: Cannot store at index %v if no value for key %v at %v",
				currentSequenceNumber+1,
				key,
				currentSequenceNumber,
			)
		}
	}

	// Remap the events into KVS events.
	remapped, errRemap := assignEventKeys(key, currentSequenceNumber, registry, events)
	if errRemap != nil {
		return errRemap
	}

	// Perform the actual put
	errCommit := store.options.PutEvents(remapped)

	return errCommit
}

// Refresh updates an aggregate with events from the store and brings it up to
// date, allowing us to work with the data.
func (store *store) Refresh(loader eventsourcing.StoreLoaderAdapter) error {
	key := loader.GetKey()

	// If the aggregate is dirty, prevent refresh from occurring.
	if loader.IsDirty() {
		return fmt.Errorf("StoreError: Aggregate %v is modified", key)
	}

	reg := loader.GetEventRegistry()
	seq := loader.SequenceNumber()

	loaded, errLoad := store.options.FetchEvents(key, seq)
	if errLoad != nil {
		return errLoad
	}

	// Rehydate events
	toApply := make([]eventsourcing.Event, len(loaded))
	for index, event := range loaded {
		summoned := reg.CreateEvent(event.EventType)
		config := &mapstructure.DecoderConfig{
			TagName:          "json",
			Result:           summoned,
			WeaklyTypedInput: true,
		}
		decoder, errDecoder := mapstructure.NewDecoder(config)
		if errDecoder != nil {
			return errDecoder
		}

		errDecode := decoder.Decode(event.EventData)
		if errDecode != nil {
			return errDecode
		}

		// Standard reflection voodoo.
		slice := reflect.ValueOf(toApply)
		target := slice.Index(index)
		target.Set(reflect.ValueOf(summoned).Elem())
	}

	// Apply
	for _, eventTyped := range toApply {
		loader.ReplayEvent(eventTyped)
	}

	return nil
}

// assignEventKeys converts keyless events into keyed store events.
func assignEventKeys(key string, seq int64, registry eventsourcing.EventRegistry, events []eventsourcing.Event) ([]KeyedEvent, error) {
	target := make([]KeyedEvent, len(events))
	for index, value := range events {
		eventName, found := registry.GetEventType(value)
		if !found {
			return nil, fmt.Errorf("Could not find specified event type for %v (initial=%v)", key, seq)
		}

		target[index] = KeyedEvent{
			Key:       key,
			Sequence:  seq + int64(1+index),
			EventType: eventName,
			EventData: value,
		}
	}

	return target, nil
}
