package inmemory

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/go-gadgets/eventsourcing"
)

// NewStore creates a new in memory event store.
func NewStore() eventsourcing.EventStore {
	return &store{
		streams: make(map[string][]item),
	}
}

// store represents an ephemeral event storage provider, storing events in-memory for the
// purposes of supporting unit tests, or disposable-data scenarios.
type store struct {
	// streams is a map of string-serialized event streams. This is to ensure
	// that we are actually round-tripping to a non-native object, rather
	// that storing instances directly or by pointers
	streams map[string][]item
}

// item represents an item in the store.
type item struct {
	// eventType is the type of event
	eventType eventsourcing.EventType

	// body is the body of the event being stored, using encoding/json
	body []byte
}

// loadEvents loads events for the specified aggregate that are beyond the
// specified baseSequence number.
func (store *store) loadEvents(key string, registry eventsourcing.EventRegistry, baseSequence int64) ([]eventsourcing.Event, error) {
	// No stream, empty list.
	stream, hasStream := store.streams[key]
	if !hasStream {
		return make([]eventsourcing.Event, 0), nil
	}

	// Create our events set
	result := make([]eventsourcing.Event, len(stream))

	// Iterate events
	for index, item := range stream {
		// Skip anything less than what we're after.
		if index < int(baseSequence) {
			continue
		}

		// Summon a blank event
		itemEventType := eventsourcing.EventType(item.eventType)
		summonedType := registry.CreateEvent(itemEventType)

		// Store the event
		errUnmarshal := json.Unmarshal(item.body, &summonedType)
		if errUnmarshal != nil {
			return nil, errUnmarshal
		}

		// This reflection evil is needed because we are now trying
		// to turn the event structure pointer back into a raw instance
		// of the structure, so as to maintain immutability/pass-by-value.
		slice := reflect.ValueOf(result)
		target := slice.Index(index)
		target.Set(reflect.ValueOf(summonedType).Elem())
	}

	return result, nil
}

// CommitEvents stores the events associated with the specified aggregate.
func (store *store) CommitEvents(adapter eventsourcing.StoreWriterAdapter) error {
	registry := adapter.GetEventRegistry()
	key := adapter.GetKey()
	initialSequence, uncommitted := adapter.GetUncomittedEvents()

	// If no uncommitted events, exit
	if len(uncommitted) == 0 {
		return nil
	}

	// Get the stream
	stream, ok := store.streams[key]

	// If the stream does not exist...
	if !ok {
		// Thats fine, for sequence 0
		if initialSequence == 0 {
			stream = make([]item, 0)
		} else {
			// But we can't append to non-zero index of empty stream
			return fmt.Errorf(
				"StoreError: Cannot store events at %v for key %v: the stream does not exist",
				initialSequence, key)
		}
	} else {
		// If we're appending far beyond the end of the stream....
		initialSequenceInt := int(initialSequence)
		if initialSequenceInt > len(stream) {
			return fmt.Errorf(
				"StoreError: Cannot store events at %v for key %v: the stream is only %v long",
				initialSequence,
				key,
				len(stream),
			)
		}

		// If we are writing to position that's already
		// occupied, then throw a concurrency fault
		if initialSequenceInt < len(stream) {
			return eventsourcing.NewConcurrencyFault(key, initialSequence)
		}
	}

	// We're all good now, lets write the events
	for _, event := range uncommitted {
		// Event type mapping
		eventType, found := registry.GetEventType(event)
		if !found {
			return fmt.Errorf("StoreError: Could not find event type to store: %v", event)
		}

		// Encode the event
		encoded, errMarshal := json.Marshal(event)
		if errMarshal != nil {
			return errMarshal
		}

		// Append the event
		stream = append(stream, item{
			eventType: eventType,
			body:      encoded,
		})
	}

	store.streams[key] = stream

	return nil
}

// Refresh the state of the aggregate from the store.
func (store *store) Refresh(adapter eventsourcing.StoreLoaderAdapter) error {
	key := adapter.GetKey()
	reg := adapter.GetEventRegistry()
	seq := adapter.SequenceNumber()

	// If the aggregate is dirty, prevent refresh from occurring.
	if adapter.IsDirty() {
		return fmt.Errorf("StoreError: Aggregate %v is modified", key)
	}

	// Load the events
	events, errEvents := store.loadEvents(key, reg, seq)
	if errEvents != nil {
		return errEvents
	}

	// Apply each event to bring the state back to current
	for _, evt := range events {
		adapter.ReplayEvent(evt)
	}

	return nil
}
