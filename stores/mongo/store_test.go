package mongo

import (
	"fmt"
	"os"
	"testing"

	"github.com/go-gadgets/eventsourcing"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
)

type MongoStoreWriter struct {
	key              string
	eventRegistry    eventsourcing.EventRegistry
	uncomittedOrigin int64
	uncomittedEvents []interface{}
}

// GetState fetches the aggregate state. In this test, it's a dummy implementation
func (instance *MongoStoreWriter) GetState() interface{} {
	return map[string]interface{}{}
}

// GetKey gets the key of the aggregate being stored
func (instance *MongoStoreWriter) GetKey() string {
	return instance.key
}

// GetEventRegistry gets the event registry for the events being stored
func (instance *MongoStoreWriter) GetEventRegistry() eventsourcing.EventRegistry {
	return instance.eventRegistry
}

// GetUncomittedEvents gets the events that are uncomittedEvents for this aggregate.
func (instance *MongoStoreWriter) GetUncomittedEvents() (int64, []interface{}) {
	return instance.uncomittedOrigin, instance.uncomittedEvents
}

// IsDirty is true if there are uncomitted events
func (instance *MongoStoreWriter) IsDirty() bool {
	return len(instance.uncomittedEvents) > 0
}

// SequenceNumber returns the sequence number of the aggregate now
func (instance *MongoStoreWriter) SequenceNumber() int64 {
	return instance.uncomittedOrigin + int64(len(instance.uncomittedEvents))
}

// CreateTestMongoStore creates a test store that talks to MongoDB
func CreateTestMongoStore() (eventsourcing.EventStore, func(), error) {
	collectionName := fmt.Sprintf("%s", uuid.NewV4())

	dial := os.Getenv("MONGO_TEST_HOST")
	if dial == "" {
		dial = "mongodb://localhost:27017"
	}

	result, err := NewMongoStore(StoreParameters{
		DialURL:        dial,
		DatabaseName:   "TestDatabase",
		CollectionName: collectionName,
	})

	if err != nil {
		return nil, nil, err
	}

	return result, func() {
		result.(*mongoDBEventStore).database.DropDatabase()
	}, nil
}

// TestMongoStoreEventStorage checks that the events are stored and loaded as expected from
// the specified event-store.
func TestMongoStoreEventStorage(t *testing.T) {
	instance := &TestOnlyAggregate{}
	store, cleanup, errStore := CreateTestMongoStore()
	if errStore != nil {
		t.Error(errStore)
		return
	}
	defer cleanup()

	// 1: Get event
	instance.Initialize("dummy-key", counterRegistry, store)
	instance.Refresh()

	// 2: Apply event
	instance.Commit()
	instance.ApplyEvent(InitializeEvent{
		TargetValue: 3,
	})

	// 3: Commit new events
	instance.Commit()

	// 4: Reload
	reloaded := &TestOnlyAggregate{}
	reloaded.Initialize("dummy-key", counterRegistry, store)
	assert.Equal(t, 0, reloaded.TargetValue, "The aggregate target value should be unset")
	reloaded.Refresh()
	assert.Equal(t, 3, reloaded.TargetValue, "The aggregate target value should be 3")
}

// TestMongoStoreAppendNonExist checks that you can't append events past
// sequence 0 for an aggregate that does not exist.
func TestMongoStoreAppendNonExist(t *testing.T) {
	store, cleanup, errStore := CreateTestMongoStore()
	if errStore != nil {
		t.Error(errStore)
		return
	}
	defer cleanup()
	exampleEvents := make([]interface{}, 1)
	exampleEvents[0] =
		InitializeEvent{
			TargetValue: 3,
		}
	err := store.CommitEvents(&MongoStoreWriter{
		key:              "dummy-key",
		eventRegistry:    counterRegistry,
		uncomittedOrigin: 1,
		uncomittedEvents: exampleEvents,
	})
	assert.NotNil(t, err, "The event store should fail when trying to write past 0 on a new entity.")
}

// TestMongoStoreAppendPastEnd checks that you can't append events past
// the end for an aggregate that does exist.
func TestMongoStoreAppendPastEnd(t *testing.T) {
	store, cleanup, errStore := CreateTestMongoStore()
	if errStore != nil {
		t.Error(errStore)
		return
	}
	defer cleanup()
	exampleEvents := make([]interface{}, 1)
	exampleEvents[0] =
		InitializeEvent{
			TargetValue: 3,
		}
	err := store.CommitEvents(&MongoStoreWriter{
		key:              "dummy-key",
		eventRegistry:    counterRegistry,
		uncomittedOrigin: 0,
		uncomittedEvents: exampleEvents,
	})
	assert.Nil(t, err, "The initial commit at 0 should succeed.")
	err = store.CommitEvents(&MongoStoreWriter{
		key:              "dummy-key",
		eventRegistry:    counterRegistry,
		uncomittedOrigin: 2,
		uncomittedEvents: exampleEvents,
	})
	assert.NotNil(t, err, "The event store should fail when trying to write past 0 on a new entity.")
}

// TestMongoStoreErrorOnUnmapped checks that you can't call store if you've
// put events in the queue that are not mapped.
func TestMongoStoreErrorOnUnmapped(t *testing.T) {
	registry := eventsourcing.NewStandardEventRegistry()
	store, cleanup, errStore := CreateTestMongoStore()
	if errStore != nil {
		t.Error(errStore)
		return
	}
	defer cleanup()
	exampleEvents := make([]interface{}, 1)
	exampleEvents[0] =
		UnknownEventTypeExample{}
	err := store.CommitEvents(&MongoStoreWriter{
		key:              "dummy-key",
		eventRegistry:    registry,
		uncomittedOrigin: 0,
		uncomittedEvents: exampleEvents,
	})
	assert.NotNil(t, err, "The event store should fail when trying to write an event that isn't in the registry.")
}

// TestMongoStoreConcurrency checks that you can't store two events at the same offset.
func TestMongoStoreConcurrency(t *testing.T) {
	store, cleanup, errStore := CreateTestMongoStore()
	if errStore != nil {
		t.Error(errStore)
		return
	}
	defer cleanup()

	firstInstance := &TestOnlyAggregate{}
	firstInstance.Initialize("dummy-key", counterRegistry, store)
	firstInstance.Refresh()
	firstInstance.ApplyEvent(InitializeEvent{
		TargetValue: 3,
	})

	secondInstance := &TestOnlyAggregate{}
	secondInstance.Initialize("dummy-key", counterRegistry, store)
	secondInstance.Refresh()
	secondInstance.ApplyEvent(InitializeEvent{
		TargetValue: 5,
	})

	firstCommitErr := firstInstance.Commit()
	assert.Nil(t, firstCommitErr, "The first commit should succeed.")

	secondCommitErr := secondInstance.Commit()
	assert.NotNil(t, secondCommitErr, "The second commit should fail.")
	isFault, _ := eventsourcing.IsConcurrencyFault(secondCommitErr)
	assert.True(t, isFault, "The second commit should ConcurencyFault")
}

// BenchmarkMongoStoreIndividualEventReplay tests how fast we can apply events to an aggregate
func BenchmarkMongoStoreIndividualEventCommit(b *testing.B) {
	instance := &TestOnlyAggregate{}
	store, cleanup, errStore := CreateTestMongoStore()
	if errStore != nil {
		b.Error(errStore)
		return
	}
	defer cleanup()
	instance.Initialize("dummy-key", counterRegistry, store)

	for i := 0; i < b.N; i++ {
		instance.ApplyEvent(IncrementEvent{
			IncrementBy: 1,
		})
		err := instance.Commit()
		if err != nil {
			b.Error(err)
		}
	}
}

// BenchmarkMongoStoreLoad1000Events tests how fast we can write
// and then load/refresh 1000 events from an aggregate
func BenchmarkMongoStoreLoad1000Events(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Create the aggregate
		store, cleanup, errStore := CreateTestMongoStore()
		if errStore != nil {
			b.Error(errStore)
			return
		}
		defer cleanup()
		instance := &TestOnlyAggregate{}
		instance.Initialize("dummy-key", counterRegistry, store)
		instance.Refresh()
		for x := 0; x < 1000; x++ {
			instance.ApplyEvent(IncrementEvent{
				IncrementBy: 1,
			})
		}
		instance.Commit()

		// Reload
		reload := &TestOnlyAggregate{}
		reload.Initialize("dummy-key", counterRegistry, store)
		reload.Refresh()
		assert.Equal(b, 1000, instance.CurrentCount)
	}
}
