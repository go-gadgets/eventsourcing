package mongo

import (
	"fmt"
	"strings"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/stores/key-value"
)

// mongoDBEventStore is a type that represents a MongoDB backed
// EventStore implementation
type mongoDBEventStore struct {
	session    *mgo.Session
	collection *mgo.Collection
}

// Endpoint are parameters for the MongoDB event store
// to use when initializing.
type Endpoint struct {
	DialURL        string `json:"dial_url"`        // DialURL is the mgo URL to use when connecting to the cluster
	DatabaseName   string `json:"database_name"`   // DatabaseName is the database to create/connect to.
	CollectionName string `json:"collection_name"` // CollectionName is the collection name to put new documents in to
}

// NewStore creates a new MongoDB backed event store for an
// application to use.
func NewStore(endpoint Endpoint) (eventsourcing.EventStore, error) {
	// Connect to the MongoDB services
	session, errSession := mgo.Dial(endpoint.DialURL)
	if errSession != nil {
		return nil, errSession
	}

	database := session.DB(endpoint.DatabaseName)
	collection := database.C(endpoint.CollectionName)

	return NewStoreWithConnection(session, collection)
}

// NewStoreWithConnection creates a new MGO-backed store with a specific session
// and collection. The collection is used to store the records, the session is used
// to clean up afterward.
func NewStoreWithConnection(session *mgo.Session, collection *mgo.Collection) (eventsourcing.EventStore, error) {
	// Validate BSON tag fallback global state
	if !bson.JSONTagFallbackState() {
		return nil, fmt.Errorf("You must configure mgo with bson.SetJSONTagFallback(true) to use this driver")
	}

	// Ensure the index exists
	errIndex := collection.EnsureIndex(mgo.Index{
		Key:        []string{"key", "sequence"},
		Unique:     true,
		DropDups:   false,
		Background: false,
	})
	if errIndex != nil {
		session.Close()
		return nil, errIndex
	}

	engine := &mongoDBEventStore{
		session:    session,
		collection: collection,
	}

	store := keyvalue.NewStore(keyvalue.Options{
		CheckSequence: engine.checkExists,
		FetchEvents:   engine.fetchEvents,
		PutEvents:     engine.putEvents,
		Close: func() error {
			session.Close()
			return nil
		},
	})

	return store, nil
}

// checkExists checks that a particular sequence number exists in the store.
func (store *mongoDBEventStore) checkExists(key string, seq int64) (bool, error) {
	var result []interface{}
	errSequence := store.collection.Find(bson.M{
		"key":      key,
		"sequence": seq,
	}).All(&result)

	return result != nil && len(result) == 1, errSequence
}

// putEvents writes events to the backing store.
func (store *mongoDBEventStore) putEvents(events []keyvalue.KeyedEvent) error {
	bulk := store.collection.Bulk()
	for _, event := range events {
		bulk.Insert(event)
	}
	_, errBulk := bulk.Run()

	if errBulk != nil && strings.HasPrefix(errBulk.Error(), "E11000") {
		firstEvent := events[0]
		return eventsourcing.NewConcurrencyFault(firstEvent.Key, firstEvent.Sequence)
	}

	return errBulk
}

// Fetch events from the Mongo store
func (store *mongoDBEventStore) fetchEvents(key string, seq int64) ([]keyvalue.KeyedEvent, error) {
	// Load the events from mgo
	var loaded []keyvalue.KeyedEvent
	errLoad := store.collection.Find(
		bson.M{
			"key": key,
			"sequence": bson.M{
				"$gt": seq,
			},
		},
	).Sort("sequence").All(&loaded)

	if errLoad != nil {
		return nil, errLoad
	}

	return loaded, nil
}
