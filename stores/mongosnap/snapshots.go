package mongosnap

import (
	"github.com/go-gadgets/eventsourcing"
	mgo "github.com/steve-gray/mgo-eventsourcing"
	"github.com/steve-gray/mgo-eventsourcing/bson"
)

// Snapshot is the current snapshot for an entity, a JSON structure
// that can be persisted to the Mongo instance.
type snapshot struct {
	Key      string      `json:"_id"`
	Sequence int64       `json:"sequence"`
	State    interface{} `json:"state"`
}

// snapStore is a type that represents a MongoDB backed
// EventStore wrapper that snapshots an aggregate at intervals
// to reduce event replay costs. This type is private to prevent
// people playing with the internals.
type snapStore struct {
	session      *mgo.Session
	database     *mgo.Database
	collection   *mgo.Collection
	snapInterval int64
	eventStore   eventsourcing.EventStore
}

// Parameters describes the parameters that can be
// used to cofigure a MongoDB snap store.
type Parameters struct {
	DialURL        string `json:"dial_url"`        // DialURL is the mgo URL to use when connecting to the cluster
	DatabaseName   string `json:"database_name"`   // DatabaseName is the database to create/connect to.
	CollectionName string `json:"collection_name"` // CollectionName is the collection name to put new documents in to
	SnapInterval   int64  `json:"snap_interval"`   // SnapInterval is the number of events between snaps
}

// NewStore creates a a new instance of the MongoDB backed snapshot provider,
// which provides aggregate replay acceleration for long-lived entities.
func NewStore(params Parameters, wrapped eventsourcing.EventStore) (eventsourcing.EventStore, error) {
	// Connect to the MongoDB services
	session, errSession := mgo.Dial(params.DialURL)
	if errSession != nil {
		return nil, errSession
	}

	database := session.DB(params.DatabaseName)
	collection := database.C(params.CollectionName)
	errIndex := collection.EnsureIndex(mgo.Index{
		Key:        []string{"key"},
		Unique:     true,
		DropDups:   false,
		Background: false,
	})
	if errIndex != nil {
		session.Close()
		return nil, errIndex
	}

	return &snapStore{
		session:      session,
		database:     database,
		collection:   collection,
		eventStore:   wrapped,
		snapInterval: params.SnapInterval,
	}, nil
}

// Close the snap-store
func (store *snapStore) Close() error {
	return nil
}

// CommitEvents stores any events for the specified aggregate that are uncommitted
// at this point in time.
func (store *snapStore) CommitEvents(writer eventsourcing.StoreWriterAdapter) error {
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
	if !writeSnap {
		return nil
	}

	// Finally, write the snap if needed
	key := writer.GetKey()
	_, errSnap := store.collection.UpsertId(key, snapshot{
		Key:      key,
		Sequence: currentSequenceNumber + eventCount,
		State:    writer.GetState(),
	})

	return errSnap
}

// Refresh the state of an aggregate from the store.
func (store *snapStore) Refresh(adapter eventsourcing.StoreLoaderAdapter) error {
	key := adapter.GetKey()

	// Only run the the snapshot fetch if an outer-driver has not already refreshed
	// the aggregate state. This allows for composition of snap-drivers.
	if adapter.SequenceNumber() == 0 {
		// Load the events from mgo
		var loaded snapshot
		errLoad := store.collection.Find(
			bson.M{
				"_id": key,
			},
		).
			Sort("-sequence").
			One(&loaded)

		if errLoad != nil && errLoad != mgo.ErrNotFound {
			return errLoad
		}

		// Restore the snapshotted state
		if loaded.Sequence > 0 {
			errSnap := adapter.RestoreSnapshot(loaded.Sequence, loaded.State)
			if errSnap != nil {
				return nil
			}
		}
	}

	// Now we can run the inner adapters refresh, andload in any
	// subsequent events that are not part of the snap.
	return store.eventStore.Refresh(adapter)
}
