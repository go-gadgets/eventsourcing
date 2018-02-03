package mongo

import (
	"strings"

	"github.com/go-gadgets/eventsourcing"
	keyvalue "github.com/go-gadgets/eventsourcing/stores/key-value"
	"github.com/steve-gray/mgo-eventsourcing"
	"github.com/steve-gray/mgo-eventsourcing/bson"
)

func init() {
	bson.SetJSONFallback(true)
}

// mongoDBEventStore is a type that represents a MongoDB backed
// EventStore implementation
type mongoDBEventStore struct {
	session    *mgo.Session
	database   *mgo.Database
	collection *mgo.Collection
}

// StoreParameters are parameters for the MongoDB event store
// to use when initializing.
type StoreParameters struct {
	DialURL        string `json:"dial_url"`        // DialURL is the mgo URL to use when connecting to the cluster
	DatabaseName   string `json:"database_name"`   // DatabaseName is the database to create/connect to.
	CollectionName string `json:"collection_name"` // CollectionName is the collection name to put new documents in to
}

// NewStore creates a new MongoDB backed event store for an
// application to use.
func NewStore(params StoreParameters) (eventsourcing.EventStore, error) {
	// Connect to the MongoDB services
	session, errSession := mgo.Dial(params.DialURL)
	if errSession != nil {
		return nil, errSession
	}

	database := session.DB(params.DatabaseName)
	collection := database.C(params.CollectionName)
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
		database:   database,
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
