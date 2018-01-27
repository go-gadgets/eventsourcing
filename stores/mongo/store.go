package mongo

import (
	"fmt"
	"reflect"

	"strings"

	"github.com/go-gadgets/eventsourcing"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// mongoDBEventStore is a type that represents a MongoDB backed
// EventStore implementation
type mongoDBEventStore struct {
	session    *mgo.Session
	database   *mgo.Database
	collection *mgo.Collection
}

// mongoDBEvent is the event record type we store into the collection
type mongoDBEvent struct {
	Key       string                  `json:"key"`
	Sequence  int64                   `json:"sequence"`
	EventType eventsourcing.EventType `json:"event_type" bson:"event_type"`
	EventData interface{}             `json:"event_data" bson:"event_data"`
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

	return &mongoDBEventStore{
		session:    session,
		database:   database,
		collection: collection,
	}, nil
}

// CommitEvents stores any events for the specified aggregate that are uncommitted
// at this point in time.
func (store *mongoDBEventStore) CommitEvents(writer eventsourcing.StoreWriterAdapter) error {
	key := writer.GetKey()
	registry := writer.GetEventRegistry()
	currentSequenceNumber, events := writer.GetUncomittedEvents()

	// If the current sequence number is > 0, check that
	// we've got a previous event stored.
	if currentSequenceNumber > 0 {
		var result []interface{}
		errPrevious := store.collection.Find(bson.M{
			"key":      key,
			"sequence": currentSequenceNumber,
		}).All(&result)
		if errPrevious != nil {
			return errPrevious
		}
		if len(result) != 1 {
			return fmt.Errorf(
				"StoreError: Cannot store at index %v if no value for key %v at %v",
				currentSequenceNumber+1,
				key,
				currentSequenceNumber,
			)
		}
	}

	// Prepare a bulk operation, mgo's documents are terrible for this
	// but I've found example code at http://grokbase.com/t/gg/mgo-users/14ab5yrtb9/mgo-bulk-example
	bulk := store.collection.Bulk()
	for index, rawEvent := range events {
		itemSequence := currentSequenceNumber + int64(1+index)
		eventName, found := registry.GetEventType(rawEvent)
		if !found {
			return fmt.Errorf("StoreError: Cannot store event, the type is not recognized: %v @ %v",
				key,
				itemSequence)
		}

		bulk.Insert(mongoDBEvent{
			Key:       key,
			Sequence:  itemSequence,
			EventType: eventName,
			EventData: rawEvent,
		})
	}

	_, errBulk := bulk.Run()
	if errBulk != nil && strings.HasPrefix(errBulk.Error(), "E11000") {
		return eventsourcing.NewConcurrencyFault(key, currentSequenceNumber+1)
	}
	return errBulk
}

// Refresh the state of an aggregate from the store.
func (store *mongoDBEventStore) Refresh(adapter eventsourcing.StoreLoaderAdapter) error {
	key := adapter.GetKey()
	reg := adapter.GetEventRegistry()
	seq := adapter.SequenceNumber()

	// If the aggregate is dirty, prevent refresh from occurring.
	if adapter.IsDirty() {
		return fmt.Errorf("StoreError: Aggregate %v is modified", key)
	}

	// Load the events from mgo
	var loaded []mongoDBEvent
	errLoad := store.collection.Find(
		bson.M{
			"key": key,
			"sequence": bson.M{
				"$gt": seq,
			},
		},
	).Sort("sequence").All(&loaded)

	if errLoad != nil {
		return errLoad
	}

	// Rehydate events
	toApply := make([]interface{}, len(loaded))
	for index, event := range loaded {
		summoned := reg.CreateEvent(event.EventType)
		errDecode := mapstructure.Decode(event.EventData, summoned)
		if errDecode != nil {
			return errDecode
		}

		// Standard reflection voodoo.
		slice := reflect.ValueOf(toApply)
		target := slice.Index(index)
		target.Set(reflect.ValueOf(summoned).Elem())
	}

	// Apply
	for _, eventTyped := range toApply {
		adapter.ReplayEvent(eventTyped)
	}

	return nil
}
