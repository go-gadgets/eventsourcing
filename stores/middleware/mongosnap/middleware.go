package mongosnap

import (
	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/stores/middleware/snapbase"
	mgo "github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
)

func init() {
	bson.SetJSONTagFallback(true)
}

// Snapshot is the current snapshot for an entity, a JSON structure
// that can be persisted to the Mongo instance.
type snapshot struct {
	Sequence int64       `json:"sequence"`
	State    interface{} `json:"state"`
}

// Endpoint configuration
type Endpoint struct {
	DialURL        string `json:"dial_url"`        // DialURL is the mgo URL to use when connecting to the cluster
	DatabaseName   string `json:"database_name"`   // DatabaseName is the database to create/connect to.
	CollectionName string `json:"collection_name"` // CollectionName is the collection name to put new documents in to
}

// Parameters describes the parameters that can be
// used to cofigure a MongoDB snap store.
type Parameters struct {
	Lazy         bool  // Lazy mode?
	SnapInterval int64 `json:"snap_interval"` // SnapInterval is the number of events between snaps
}

// instance is our storage provider for managing snapshots in memory
type instance struct {
	session    *mgo.Session
	collection *mgo.Collection
	params     Parameters
}

// Create provisions a new instance of the memory-snap provider.
func Create(params Parameters, endpoint Endpoint) (eventsourcing.MiddlewareFactory, error) {
	// Connect to the MongoDB services
	session, errSession := mgo.Dial(endpoint.DialURL)
	if errSession != nil {
		return nil, errSession
	}

	database := session.DB(endpoint.DatabaseName)
	collection := database.C(endpoint.CollectionName)

	return CreateWithConnection(params, session, collection), nil
}

// CreateWithConnection provisions a new instance of the memory-snap provider using
// an existing connection and session
func CreateWithConnection(params Parameters, session *mgo.Session, collection *mgo.Collection) eventsourcing.MiddlewareFactory {
	snaps := &instance{
		session:    session,
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
	}
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
