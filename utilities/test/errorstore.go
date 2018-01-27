package test

import "github.com/go-gadgets/eventsourcing"

// The errorStore is an event store that returns a defined error each time.
type errorStore struct {
	// errorToReturn is the error that gets returned each call
	errorToReturn error
}

// CreateErrorStore creates a store that fails every operation with an error.
func CreateErrorStore(err error) eventsourcing.EventStore {
	return &errorStore{
		errorToReturn: err,
	}
}

// CommitEvents writes events to a backing store. However, for the errorStore we
// simply return an error.
func (store *errorStore) CommitEvents(adapter eventsourcing.StoreWriterAdapter) error {
	return store.errorToReturn
}

// Refresh the state of the aggregate from the store. Returns an error.
func (store *errorStore) Refresh(adapter eventsourcing.StoreLoaderAdapter) error {
	return store.errorToReturn
}
