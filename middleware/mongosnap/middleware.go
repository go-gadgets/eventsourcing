package mongosnap

import (
	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/middleware/snapbase"
	mgo "github.com/steve-gray/mgo-eventsourcing"
	"github.com/steve-gray/mgo-eventsourcing/bson"
)

func init() {
	bson.SetJSONFallback(true)
}

// Snapshot is the current snapshot for an entity, a JSON structure
// that can be persisted to the Mongo instance.
type snapshot struct {
	Sequence int64       `json:"sequence"`
	State    interface{} `json:"state"`
}

// Parameters describes the parameters that can be
// used to cofigure a MongoDB snap store.
type Parameters struct {
	DialURL        string `json:"dial_url"`        // DialURL is the mgo URL to use when connecting to the cluster
	DatabaseName   string `json:"database_name"`   // DatabaseName is the database to create/connect to.
	CollectionName string `json:"collection_name"` // CollectionName is the collection name to put new documents in to
	Lazy           bool   // Lazy mode?
	SnapInterval   int64  `json:"snap_interval"` // SnapInterval is the number of events between snaps
}

// instance is our storage provider for managing snapshots in memory
type instance struct {
	session    *mgo.Session
	database   *mgo.Database
	collection *mgo.Collection
	params     Parameters
}

// Create provisions a new instance of the memory-snap provider.
func Create(params Parameters) (eventsourcing.MiddlewareFactory, error) {
	// Connect to the MongoDB services
	session, errSession := mgo.Dial(params.DialURL)
	if errSession != nil {
		return nil, errSession
	}

	database := session.DB(params.DatabaseName)
	collection := database.C(params.CollectionName)

	snaps := &instance{
		session:    session,
		database:   database,
		collection: collection,
		params:     params,
	}

	return func() (eventsourcing.CommitMiddleware, eventsourcing.RefreshMiddleware, eventsourcing.CloseMiddleware) {
		return snapbase.Create(snapbase.Parameters{
			Lazy:         params.Lazy,
			SnapInterval: params.SnapInterval,
			Close: func() error {
				session.Close()
				return nil
			},
			Get:   snaps.get,
			Purge: snaps.purge,
			Put:   snaps.put,
		})
	}, nil
}

// get a key from the cache
func (mw *instance) get(key string) (interface{}, int64, error) {
	var loaded snapshot
	errLoad := mw.collection.Find(
		bson.M{
			"_id": key,
		},
	).
		Sort("-sequence").
		One(&loaded)

	if errLoad != nil && errLoad != mgo.ErrNotFound {
		return nil, 0, errLoad
	}

	return loaded.State, loaded.Sequence, nil
}

// purge a key from the cache
func (mw *instance) purge(key string) error {
	errPurge := mw.collection.Remove(
		bson.M{
			"_id": key,
		},
	)
	return errPurge
}

// put an item into the cache
func (mw *instance) put(key string, seq int64, data interface{}) error {
	_, errSnap := mw.collection.UpsertId(key, snapshot{
		Sequence: seq,
		State:    data,
	})

	return errSnap
}
