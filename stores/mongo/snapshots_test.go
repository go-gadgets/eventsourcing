package mongo

import (
	"fmt"
	"os"
	"testing"

	"github.com/go-gadgets/eventsourcing"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
)

// CreateSnapStore creates a test store that talks to MongoDB
func CreateSnapStore() (eventsourcing.EventStore, *eventsourcing.TestStore, func(), error) {
	collectionName := fmt.Sprintf("%s", uuid.NewV4())

	dial := os.Getenv("MONGO_TEST_HOST")
	if dial == "" {
		dial = "mongodb://localhost:27017"
	}

	inner := eventsourcing.NewTestStore()

	outer, err := NewSnapStore(SnapParameters{
		DialURL:        dial,
		DatabaseName:   "TestDatabase",
		CollectionName: collectionName + "-Snapshot",
		SnapInterval:   3,
	}, inner)
	if err != nil {
		return nil, nil, nil, err
	}

	return outer, inner, func() {
		outer.(*snapStore).database.DropDatabase()
	}, nil
}

// TestSnapBasicSnapAndRecovery performs a basic snap and recover cycle, pushing
// through several events (enough to pass the SnapInterval) on a clean aggregate.
func TestSnapBasicSnapAndRecovery(t *testing.T) {
	// Test Setup
	snap, inner, cleanup, errStore := CreateSnapStore()
	if errStore != nil {
		t.Error(errStore)
		return
	}
	defer cleanup()

	// Push the snap
	err := snap.CommitEvents(&MongoStoreWriter{
		key:              "dummy-key",
		eventRegistry:    counterRegistry,
		uncomittedOrigin: 0,
		uncomittedEvents: []eventsourcing.Event{
			InitializeEvent{TargetValue: 3}, // 0
			IncrementEvent{IncrementBy: 1},  // 1
			IncrementEvent{IncrementBy: 1},  // 2
		},
		state: &TestOnlyAggregate{
			CurrentCount: 2,
			TargetValue:  3,
		},
	})
	if err != nil {
		t.Error(err)
		return
	}

	// Now stage our mock so we can only delta-load from position 3
	inner.When("dummy-key", int64(3), []eventsourcing.Event{
		IncrementEvent{IncrementBy: 1}, // Event 3
	}, nil)

	// And let's recover.
	instance := TestOnlyAggregate{}
	instance.Initialize("dummy-key", counterRegistry, snap, func() interface{} { return &instance })
	errRefresh := instance.Refresh()
	if errRefresh != nil {
		t.Error(errRefresh)
	}

	assert.EqualValues(t, 3, instance.CurrentCount, "Should have a Count=3")
}
