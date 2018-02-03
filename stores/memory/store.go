package memory

import (
	"encoding/json"

	"github.com/go-gadgets/eventsourcing"
	keyvalue "github.com/go-gadgets/eventsourcing/stores/key-value"
)

// NewStore creates a new in memory event store.
func NewStore() eventsourcing.EventStore {
	provider := &state{
		streams: make(map[string][]item),
	}

	store := keyvalue.NewStore(keyvalue.Options{
		CheckSequence: provider.checkExists,
		FetchEvents:   provider.fetchEvents,
		PutEvents:     provider.putEvents,
		Close: func() error {
			provider.streams = nil
			return nil
		},
	})

	return store
}

// state contains the current data for an in-memory store.
type state struct {
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

// checkExists checks that a particular sequence number exists in the store.
func (data *state) checkExists(key string, seq int64) (bool, error) {
	stream, found := data.streams[key]
	if !found {
		return false, nil
	}

	return len(stream) >= int(seq), nil
}

// fetchEvents checks all events beyond the specified sequence number.
func (data *state) fetchEvents(key string, seq int64) ([]keyvalue.KeyedEvent, error) {
	stream, found := data.streams[key]

	// If no stream, or we've only got prior events, then return an empty
	// set of events.
	if !found || len(stream) < int(seq) {
		return []keyvalue.KeyedEvent{}, nil
	}

	result := make([]keyvalue.KeyedEvent, 0)
	for index := int(seq); index < len(stream); index++ {
		// Rehydrate the JSON
		target := make(map[string]interface{})
		errUnmarshal := json.Unmarshal(stream[index].body, &target)
		if errUnmarshal != nil {
			return nil, errUnmarshal
		}

		result = append(result, keyvalue.KeyedEvent{
			Key:       key,
			Sequence:  seq + int64(1+index),
			EventType: stream[index].eventType,
			EventData: target,
		})
	}
	return result, nil
}

// putEvents writes events to the store
func (data *state) putEvents(events []keyvalue.KeyedEvent) error {
	for _, evt := range events {
		stream, found := data.streams[evt.Key]
		if !found {
			stream = make([]item, 0)
		}

		// Concurrency check (are we inserting over the top of an event?)
		// (Event Seq=1 is array index 0)
		expectedLength := int(evt.Sequence - 1)
		if len(stream) > expectedLength {
			return eventsourcing.NewConcurrencyFault(evt.Key, evt.Sequence)
		}

		buff, errMarshal := json.Marshal(evt.EventData)
		if errMarshal != nil {
			return errMarshal
		}

		stream = append(stream, item{
			eventType: evt.EventType,
			body:      buff,
		})

		// Write back to the structure
		data.streams[evt.Key] = stream
	}

	return nil
}
