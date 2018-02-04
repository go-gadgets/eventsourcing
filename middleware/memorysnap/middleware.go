package memorysnap

import (
	"fmt"
	"sync"

	"github.com/go-gadgets/eventsourcing"
)

// Create provisions a new instance of the memory-snap provider.
func Create(params Parameters) (eventsourcing.CommitMiddleware, eventsourcing.RefreshMiddleware, func() error) {
	snaps := &snapStorage{
		snapInterval: params.SnapInterval,
		snaps:        make(map[string]snapshot),
	}

	return snaps.commit, snaps.refresh, func() error {
		snaps.snaps = nil
		return nil
	}
}

// Snapshot is the current snapshot for an entity
type snapshot struct {
	Sequence int64
	State    interface{}
}

// snapStorage is our storage provider for managing snapshots in memory
type snapStorage struct {
	snapInterval int64
	snaps        map[string]snapshot
	mutex        sync.Mutex
}

// Parameters describes the parameters that can be used to configure the snap store.
type Parameters struct {
	SnapInterval int64 `json:"snap_interval"` // SnapInterval is the number of events between snaps
}

// CommitEvents stores any events for the specified aggregate that are uncommitted
// at this point in time.
func (store *snapStorage) commit(writer eventsourcing.StoreWriterAdapter, next eventsourcing.NextHandler) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	// Store the inner provider first.
	errInner := next()
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

// refresh the state of an aggregate from the store.
func (store *snapStorage) refresh(adapter eventsourcing.StoreLoaderAdapter, next eventsourcing.NextHandler) error {
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
	return next()
}
