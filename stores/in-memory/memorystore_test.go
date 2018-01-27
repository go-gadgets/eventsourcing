package inmemory

import (
	"testing"

	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/utilities/test"
	"github.com/stretchr/testify/assert"
)

type testStoreWriter struct {
	key              string
	eventRegistry    eventsourcing.EventRegistry
	uncomittedOrigin int64
	uncomittedEvents []interface{}
}

// GetKey gets the key of the aggregate being stored
func (instance *testStoreWriter) GetKey() string {
	return instance.key
}

// GetEventRegistry gets the event registry for the events being stored
func (instance *testStoreWriter) GetEventRegistry() eventsourcing.EventRegistry {
	return instance.eventRegistry
}

// GetUncomittedEvents gets the events that are uncomittedEvents for this aggregate.
func (instance *testStoreWriter) GetUncomittedEvents() (int64, []interface{}) {
	return instance.uncomittedOrigin, instance.uncomittedEvents
}

// IsDirty is reture if there's an uncomitted event
func (instance *testStoreWriter) IsDirty() bool {
	return len(instance.uncomittedEvents) > 0
}

// SequenceNumber fetches the current sequence number
func (instance *testStoreWriter) SequenceNumber() int64 {
	return instance.uncomittedOrigin + int64(len(instance.uncomittedEvents))
}

// GetState returns an empty state map
func (instance *testStoreWriter) GetState() interface{} {
	return map[string]interface{}{}
}

// TestMemoryStoreCommitReload checks that the events are stored and loaded as
// expected from the specified event-store.
func TestMemoryStoreCommitReload(t *testing.T) {
	instance := &test.SimpleAggregate{}
	store := NewInMemoryStore()

	// 1: Get event
	instance.Initialize("dummy-key", test.GetTestRegistry(), store)
	instance.Refresh()

	// 2: Apply event
	instance.Commit()
	instance.ApplyEvent(test.InitializeEvent{
		TargetValue: 3,
	})

	// 3: Commit new events
	instance.Commit()

	// 4: Reload
	reloaded := &test.SimpleAggregate{}
	reloaded.Initialize("dummy-key", test.GetTestRegistry(), store)
	assert.Equal(t, 0, reloaded.TargetValue, "The aggregate target value should be unset")
	reloaded.Refresh()
	assert.Equal(t, int64(1), instance.SequenceNumber(), "The aggregate sequence number should be 1")
	assert.Equal(t, 3, reloaded.TargetValue, "The aggregate target value should be 3")
}

// TestMemoryStoreAppendNonExisting checks that you can't append events past
// sequence 0 for an aggregate that does not exist.
func TestMemoryStoreAppendNonExisting(t *testing.T) {
	store := NewInMemoryStore()
	exampleEvents := make([]interface{}, 1)
	exampleEvents[0] =
		test.InitializeEvent{
			TargetValue: 3,
		}
	err := store.CommitEvents(&testStoreWriter{
		key:              "dummy-key",
		eventRegistry:    test.GetTestRegistry(),
		uncomittedOrigin: 1,
		uncomittedEvents: exampleEvents,
	})
	assert.NotNil(t, err, "The event store should fail when trying to write past 0 on a new entity.")
}

// TestMemoryStoreAppendPastEnd checks that you can't append events past
// the end for an aggregate that does exist.
func TestMemoryStoreAppendPastEnd(t *testing.T) {
	store := NewInMemoryStore()
	exampleEvents := make([]interface{}, 1)
	exampleEvents[0] =
		test.InitializeEvent{
			TargetValue: 3,
		}
	err := store.CommitEvents(&testStoreWriter{
		key:              "dummy-key",
		eventRegistry:    test.GetTestRegistry(),
		uncomittedOrigin: 0,
		uncomittedEvents: exampleEvents,
	})
	assert.Nil(t, err, "The initial commit at 0 should succeed.")
	err = store.CommitEvents(&testStoreWriter{
		key:              "dummy-key",
		eventRegistry:    test.GetTestRegistry(),
		uncomittedOrigin: 2,
		uncomittedEvents: exampleEvents,
	})
	assert.NotNil(t, err, "The event store should fail when trying to write past 0 on a new entity.")
}

// TestMemoryStoreErrorOnUnmapped checks that you can't call store if you've
// put events in the queue that are not mapped.
func TestMemoryStoreErrorOnUnmapped(t *testing.T) {
	registry := eventsourcing.NewStandardEventRegistry()
	store := NewInMemoryStore()
	exampleEvents := make([]interface{}, 1)
	exampleEvents[0] =
		test.UnknownEventTypeExample{}
	err := store.CommitEvents(&testStoreWriter{
		key:              "dummy-key",
		eventRegistry:    registry,
		uncomittedOrigin: 0,
		uncomittedEvents: exampleEvents,
	})
	assert.NotNil(t, err, "The event store should fail when trying to write an event that isn't in the registry.")
}

// TestMemoryStoreConcurrency checks that you can't store two events at the same offset.
func TestMemoryStoreConcurrency(t *testing.T) {
	store := NewInMemoryStore()

	firstInstance := &test.SimpleAggregate{}
	firstInstance.Initialize("dummy-key", test.GetTestRegistry(), store)
	firstInstance.Refresh()
	firstInstance.ApplyEvent(test.InitializeEvent{
		TargetValue: 3,
	})

	secondInstance := &test.SimpleAggregate{}
	secondInstance.Initialize("dummy-key", test.GetTestRegistry(), store)
	secondInstance.Refresh()
	secondInstance.ApplyEvent(test.InitializeEvent{
		TargetValue: 5,
	})

	firstCommitErr := firstInstance.Commit()
	assert.Nil(t, firstCommitErr, "The first commit should succeed.")

	secondCommitErr := secondInstance.Commit()
	assert.NotNil(t, secondCommitErr, "The second commit should fail.")
	isFault, _ := eventsourcing.IsConcurrencyFault(secondCommitErr)
	assert.True(t, isFault, "The second commit should ConcurencyFault")
}

// BenchmarkMemoryStoreIndividualCommit tests how fast we can apply events to an aggregate
func BenchmarkMemoryStoreIndividualCommit(b *testing.B) {
	instance := &test.SimpleAggregate{}
	store := NewInMemoryStore()
	instance.Initialize("dummy-key", test.GetTestRegistry(), store)

	for i := 0; i < b.N; i++ {
		instance.ApplyEvent(test.IncrementEvent{
			IncrementBy: 1,
		})
		instance.Commit()
	}
}

// BenchmarkMemoryStoreBulkCommit tests how fast we can load/refresh 1000 events from an aggregate
func BenchmarkMemoryStoreBulkCommit(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Create the aggregate
		store := NewInMemoryStore()
		instance := &test.SimpleAggregate{}
		instance.Initialize("dummy-key", test.GetTestRegistry(), store)
		instance.Refresh()
		for x := 0; x < 1000; x++ {
			instance.ApplyEvent(test.IncrementEvent{
				IncrementBy: 1,
			})
		}
		instance.Commit()

		// Reload
		reload := &test.SimpleAggregate{}
		reload.Initialize("dummy-key", test.GetTestRegistry(), store)
		reload.Refresh()
	}
}
