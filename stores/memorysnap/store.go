package memorysnap

import (
	"fmt"
	"sync"

	"github.com/go-gadgets/eventsourcing"
)

// Snapshot is the current snapshot for an entity, a JSON structure
// that can be persisted to the Mongo instance.
type snapshot struct {
	Sequence int64       `json:"sequence"`
	State    interface{} `json:"aggregate_state"`
}

// snapStore is a type that represents a MongoDB backed
// EventStore wrapper that snapshots an aggregate at intervals
// to reduce event replay costs. This type is private to prevent
// people playing with the internals.
type snapStore struct {
	snapInterval int64
	eventStore   eventsourcing.EventStore
	snaps        map[string]snapshot
	mutex        sync.Mutex
}

// Parameters describes the parameters that can be used to configure the snap store.
type Parameters struct {
	SnapInterval int64 `json:"snap_interval"` // SnapInterval is the number of events between snaps
}

// NewStore creates a a new instance of the MongoDB backed snapshot provider,
// which provides aggregate replay acceleration for long-lived entities.
func NewStore(params Parameters, wrapped eventsourcing.EventStore) eventsourcing.EventStore {
	return &snapStore{
		eventStore:   wrapped,
		snapInterval: params.SnapInterval,
		snaps:        make(map[string]snapshot),
	}
}

// Close the event-store driver
func (store *snapStore) Close() error {
	store.snaps = nil
	return nil
}

// CommitEvents stores any events for the specified aggregate that are uncommitted
// at this point in time.
func (store *snapStore) CommitEvents(writer eventsourcing.StoreWriterAdapter) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	// Store the inner provider first.
	errInner := store.eventStore.CommitEvents(writer)
	if errInner != nil {
		return errInner
	}

	// Snap time?
	currentSequenceNumber, events := writer.GetUncomittedEvents()
	eventCount := int64(len(events))
	nextSnap := currentSequenceNumber - (currentSequenceNumber % store.snapInterval) + store.snapInterval
	writeSnap := currentSequenceNumber+eventCount >= nextSnap

	// Finally, write the snap if needed
	key := writer.GetKey()
	if writeSnap == true {
		store.snaps[key] = snapshot{
			Sequence: currentSequenceNumber + eventCount,
			State:    writer.GetState(),
		}
	}

	return nil
}

// Refresh the state of an aggregate from the store.
func (store *snapStore) Refresh(adapter eventsourcing.StoreLoaderAdapter) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	key := adapter.GetKey()

	// If the aggregate is dirty, prevent refresh from occurring.
	if adapter.IsDirty() {
		return fmt.Errorf("StoreError: Aggregate %v is modified", key)
	}

	snap, found := store.snaps[key]
	if found {
		errSnap := adapter.RestoreSnapshot(snap.Sequence, snap.State)
		if errSnap != nil {
			return nil
		}
	}

	// Now we can run the inner adapters refresh, andload in any
	// subsequent events that are not part of the snap.
	return store.eventStore.Refresh(adapter)
}
