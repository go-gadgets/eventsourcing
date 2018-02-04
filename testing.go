package eventsourcing

import (
	"fmt"
)

// TestStoreFilter is a callback that decides whether or not to fail the next
// operation with an error
type TestStoreFilter func() error

// DefaultTestStoreFilter is an error filter that doesn't fail, which is the default.
func DefaultTestStoreFilter() error {
	return nil
}

// TestStoreFailureFilter is an error filter that is used to ensure a store fails
// in the prescribed way.
func TestStoreFailureFilter(err error) TestStoreFilter {
	return func() error {
		return err
	}
}

// NewTestStore creates a new EventStore instance that uses the Mock provider. This
// allows us to track and observe actions, and can be used to validate the correctness
// of other implementations or actions in unit tests.
func NewTestStore() *TestStore {
	// This pointer-play is purely so that I can get a type
	// converson failure if the Store interface ever breaks.
	var store EventStore
	store = &TestStore{
		History:     make([]TestStoreHistoryItem, 0),
		whens:       make(map[string]whenState),
		ErrorFilter: DefaultTestStoreFilter,
	}

	return store.(*TestStore)
}

// TestStore is our mock-store type
type TestStore struct {
	History     []TestStoreHistoryItem
	ErrorFilter func() error
	whens       map[string]whenState
}

// TestStoreHistoryItem is the set of history items recorded by an event store.
type TestStoreHistoryItem struct {
	Key    string      // Key of the aggregate
	Offset int64       // Offset that this set of events is from
	Events []Event     // Events
	State  interface{} // State instance
}

// whenState is a pre-configured refresh response
type whenState struct {
	Offset int64       // Expected offset
	Events []Event     // Any events
	State  interface{} // State to apply
}

// When sets up a configured state that can be used for refreshes.
func (store *TestStore) When(key string, offset int64, events []Event, state interface{}) {
	store.whens[key] = whenState{
		Offset: offset,
		Events: events,
		State:  state,
	}
}

// Close the test store
func (store *TestStore) Close() error {
	return nil
}

// CommitEvents stores the events
func (store *TestStore) CommitEvents(writer StoreWriterAdapter) error {
	seq, evt := writer.GetUncommittedEvents()

	store.History = append(store.History, TestStoreHistoryItem{
		Key:    writer.GetKey(),
		Offset: seq,
		Events: evt,
		State:  writer.GetState(),
	})

	return nil
}

// Refresh recovers the state of an aggregate from a known state.
func (store *TestStore) Refresh(reader StoreLoaderAdapter) error {
	key := reader.GetKey()
	when, configured := store.whens[key]

	// If we're not configured, and we're at sequence 0, nothing to do
	if !configured && reader.SequenceNumber() == 0 {
		return nil
	}

	// If we're not configured, and we have a non-zero sequence number, fail.
	if !configured {
		return fmt.Errorf("Cannot Refresh when not configured, already at %v for %v", reader.SequenceNumber(), key)
	}

	// Apply the result
	if when.Events != nil {
		// If we've got events
		delta := when.Offset - reader.SequenceNumber()
		if delta != 0 {
			return fmt.Errorf("Cannot Refresh, the assumed offsets are not compatible. Have %v delta vs %v", delta, reader.SequenceNumber())
		}

		for _, evt := range when.Events {
			reader.ReplayEvent(evt)
		}
	} else if when.State != nil {
		reader.RestoreSnapshot(when.Offset, when.State)
	}

	return nil
}
