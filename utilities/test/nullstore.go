package test

import "github.com/go-gadgets/eventsourcing"

// The NullStore is a simple store implementation that does nothing besides
// clear the pending events on commit. Basically it's an event black-hole,
// and should only be used for stateless tests.
type NullStore struct {
}

// CommitEvents writes events to a backing store. However, for the NullStore we
// simply do nothing and discard anything sent in.
func (store *NullStore) CommitEvents(adapter eventsourcing.StoreWriterAdapter) error {
	return nil
}

// Refresh the state of the aggregate from the store. Does nothing and will
// never change aggregate state.
func (store *NullStore) Refresh(adapter eventsourcing.StoreLoaderAdapter) error {
	// Intentionally blank
	return nil
}

// NewNullStore creates a null-store, a store that does nothing but
// accept/discard any data.
func NewNullStore() eventsourcing.EventStore {
	return &NullStore{}
}
