package snapbase

import (
	"encoding/json"
	"fmt"

	"github.com/go-gadgets/eventsourcing"
)

// Parameters is a structure that contains the various common callbacks that
// are required for a snap-provider to work correctly, as well as any additional
// parameters.
type Parameters struct {
	Lazy         bool          // Lazy provider
	SnapInterval int64         // Frequency between snaps
	Close        CloseCallback // Close callback
	Get          GetCallback   // Get entry from snapshot storage
	Purge        PurgeCallback // Purge an entr
	Put          PutCallback   // Put entry into the snapshot storage
}

// CloseCallback is a callback that closes the inner provider
type CloseCallback func() error

// GetCallback fetches a snapshot
type GetCallback func(string) (interface{}, int64, error)

// PurgeCallback purges an entry from the store.
type PurgeCallback func(string) error

// PutCallback is the callback that writes to the store
type PutCallback func(string, int64, interface{}) error

// middleware is a structure that brings together a few elements and lets
// us use function references for the commit, refresh operations etc.
type middleware struct {
	params Parameters
}

// Create a snapbase middleware with the specified parameters
func Create(parameters Parameters) (eventsourcing.CommitMiddleware, eventsourcing.RefreshMiddleware, func() error) {
	mw := &middleware{
		params: parameters,
	}

	return mw.commit, mw.refresh, func() error {
		return parameters.Close()
	}
}

// CommitEvents stores any events for the specified aggregate that are uncommitted
// at this point in time.
func (mw *middleware) commit(writer eventsourcing.StoreWriterAdapter, next eventsourcing.NextHandler) error {
	// Store the inner provider first.
	errInner := next()

	if errInner != nil {
		// If we're a lazy commit, then clean the cache on a fault
		fault, _ := eventsourcing.IsConcurrencyFault(errInner)
		if fault && mw.params.Lazy {
			key := writer.GetKey()
			errPurge := mw.params.Purge(key)
			if errPurge != nil {
				return errPurge
			}
		}

		return errInner
	}

	// Snap time?
	currentSequenceNumber, events := writer.GetUncomittedEvents()
	eventCount := int64(len(events))
	nextSnap := currentSequenceNumber - (currentSequenceNumber % mw.params.SnapInterval) + mw.params.SnapInterval
	writeSnap := mw.params.Lazy || currentSequenceNumber+eventCount >= nextSnap
	if !writeSnap {
		return nil
	}

	// Finally, write the snap if needed
	key := writer.GetKey()

	snapped, errMarshal := json.Marshal(writer.GetState())
	if errMarshal != nil {
		return errMarshal
	}
	cloned := make(map[string]interface{})
	errClone := json.Unmarshal(snapped, &cloned)
	if errClone != nil {
		return errClone
	}

	errSnap := mw.params.Put(key, currentSequenceNumber+eventCount, cloned)
	return errSnap
}

// refresh the state of an aggregate from the store.
func (mw *middleware) refresh(adapter eventsourcing.StoreLoaderAdapter, next eventsourcing.NextHandler) error {
	key := adapter.GetKey()

	// If the aggregate is dirty, prevent refresh from occurring.
	if adapter.IsDirty() {
		return fmt.Errorf("Snap errror: Aggregate %v is modified", key)
	}

	snap, seq, errLoad := mw.params.Get(key)
	if errLoad != nil {
		return errLoad
	}

	if snap != nil {
		errSnap := adapter.RestoreSnapshot(seq, snap)
		if errSnap != nil {
			return nil
		}

		// If we're lazy, then don't call the rest of the refresh
		if mw.params.Lazy {
			return nil
		}
	}

	// Now we can run the inner adapters refresh, andload in any
	// subsequent events that are not part of the snap.
	return next()
}
