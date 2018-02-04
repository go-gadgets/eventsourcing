package memorysnap

import (
	"sync"

	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/stores/middleware/snapbase"
)

// Parameters describes the parameters that can be used to configure the snap store.
type Parameters struct {
	Lazy         bool  // Lazy snapshots (won't refresh if there's a cached copy in RAM)
	SnapInterval int64 `json:"snap_interval"` // SnapInterval is the number of events between snaps
}

// Snapshot is the current snapshot for an entity
type snapshot struct {
	Sequence int64
	State    interface{}
}

// instance is our storage provider for managing snapshots in memory
type instance struct {
	snapInterval int64
	lazy         bool
	snaps        map[string]snapshot
	mutex        sync.Mutex
}

// Create provisions a new instance of the memory-snap provider.
func Create(params Parameters) (eventsourcing.CommitMiddleware, eventsourcing.RefreshMiddleware, func() error) {
	snaps := &instance{
		lazy:         params.Lazy,
		snapInterval: params.SnapInterval,
		snaps:        make(map[string]snapshot),
	}

	return snapbase.Create(snapbase.Parameters{
		Lazy:         params.Lazy,
		SnapInterval: params.SnapInterval,
		Close: func() error {
			snaps.snaps = nil
			return nil
		},
		Get:   snaps.get,
		Purge: snaps.purge,
		Put:   snaps.put,
	})
}

// get a key from the cache
func (mw *instance) get(key string) (interface{}, int64, error) {
	mw.mutex.Lock()
	defer mw.mutex.Unlock()

	snap, found := mw.snaps[key]
	if !found {
		return nil, 0, nil
	}

	return snap.State, snap.Sequence, nil
}

// purge a key from the cache
func (mw *instance) purge(key string) error {
	mw.mutex.Lock()
	defer mw.mutex.Unlock()

	_, found := mw.snaps[key]
	if found {
		delete(mw.snaps, key)
	}
	return nil
}

// put an item into the cache
func (mw *instance) put(key string, seq int64, data interface{}) error {
	mw.mutex.Lock()
	defer mw.mutex.Unlock()

	mw.snaps[key] = snapshot{
		Sequence: seq,
		State:    data,
	}
	return nil
}
