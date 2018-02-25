package test

import (
	"fmt"
	"testing"

	"github.com/go-gadgets/eventsourcing"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
)

// A StoreProvider is a structure
type StoreProvider func() (eventsourcing.EventStore, func(), error)

// ProviderFunc is a test function that runs in the context of a provider
type ProviderFunc func(store eventsourcing.EventStore) error

func getDummyKey() string {
	return uuid.NewV4().String()
}

// execute checks a behaviour for a store implementation using a standard
// setup/teardown, calling a lambda closure with the provider instance
func execute(t *testing.T, provider StoreProvider, fn ProviderFunc) {
	store, cleanup, err := provider()
	if err != nil {
		t.Error(err)
		return
	}
	defer cleanup()

	errFn := fn(store)
	if errFn != nil {
		t.Error(errFn)
	}
}

// executeBench is the benchmark equivalent of execute
func executeBench(b *testing.B, provider StoreProvider, fn ProviderFunc) {
	store, cleanup, err := provider()
	if err != nil {
		b.Error(err)
		return
	}
	defer cleanup()

	errFn := fn(store)
	if errFn != nil {
		b.Error(errFn)
	}
}

// CheckStandardSuite performs unit testing of all of the various options.
func CheckStandardSuite(t *testing.T, name string, provider StoreProvider) {
	fmt.Printf("Running provider  compliance suite for %v.....\n", name)

	fmt.Println("  >> Startup/Shutdown")
	CheckStartupShutdown(t, provider)
	if t.Failed() {
		return
	}

	fmt.Println("  >> Write new events")
	CheckWriteReadNew(t, provider)
	if t.Failed() {
		return
	}

	fmt.Println("  >> Fail when unmapped event is passed")
	CheckUnmappedEvent(t, provider)
	if t.Failed() {
		return
	}

	fmt.Println("  >> Concurrency support")
	CheckConcurrencyValidation(t, provider)
	if t.Failed() {
		return
	}

	fmt.Println("  >> Write past EOF for aggregate")
	CheckWritePastEnd(t, provider)
	if t.Failed() {
		return
	}

	fmt.Println("  >> Check repeated read/write cycles")
	CheckWriteReadWriteRead(t, provider)
	if t.Failed() {
		return
	}

	fmt.Println("  >> Check refresh of dirty aggregate fails")
	CheckDirtyRefresh(t, provider)
}

// CheckStartupShutdown checks a store starts up and shuts down cleanly.
func CheckStartupShutdown(t *testing.T, provider StoreProvider) {
	execute(t, provider, func(eventsourcing.EventStore) error { return nil })
}

// CheckWriteReadNew validates we can write and then read-lback events
func CheckWriteReadNew(t *testing.T, provider StoreProvider) {
	execute(t, provider, func(store eventsourcing.EventStore) error {
		instance := SimpleAggregate{}
		dummyKey := getDummyKey()
		instance.Initialize(dummyKey, GetTestRegistry(), store)

		instance.ApplyEvent(InitializeEvent{
			TargetValue: 3,
		})

		errCommit := instance.Commit()
		if errCommit != nil {
			return errCommit
		}

		second := SimpleAggregate{}
		second.Initialize(dummyKey, GetTestRegistry(), store)
		second.Refresh()
		if second.TargetValue != 3 {
			return fmt.Errorf("Target value should be 3: got %v", second.TargetValue)
		}

		return nil
	})
}

// CheckUnmappedEvent confirms that the event store fails when trying to save
// an event that does not exist in the mapping registry.
func CheckUnmappedEvent(t *testing.T, provider StoreProvider) {
	execute(t, provider, func(store eventsourcing.EventStore) error {
		instance := SimpleAggregate{}
		dummyKey := getDummyKey()
		instance.Initialize(dummyKey, GetTestRegistry(), store)

		instance.ApplyEvent(UnknownEventTypeExample{})

		errCommit := instance.Commit()
		if errCommit == nil {
			return fmt.Errorf("Should have failed with unmapped event, but succeeded")
		}

		return nil
	})
}

// CheckConcurrencyValidation ensures that basic concurrency safeguards are applied when
// writing to the aggregate storage, and that we can't get interleaved operations or events.
func CheckConcurrencyValidation(t *testing.T, provider StoreProvider) {
	execute(t, provider, func(store eventsourcing.EventStore) error {
		firstInstance := SimpleAggregate{}
		dummyKey := getDummyKey()
		firstInstance.Initialize(dummyKey, GetTestRegistry(), store)
		firstInstance.Refresh()
		firstInstance.ApplyEvent(InitializeEvent{
			TargetValue: 3,
		})

		secondInstance := SimpleAggregate{}
		secondInstance.Initialize(dummyKey, GetTestRegistry(), store)
		secondInstance.Refresh()
		secondInstance.ApplyEvent(InitializeEvent{
			TargetValue: 5,
		})

		firstCommitErr := firstInstance.Commit()
		if firstCommitErr != nil {
			return firstCommitErr
		}

		secondCommitErr := secondInstance.Commit()
		if secondCommitErr == nil {
			return fmt.Errorf("Got no fault (expected a fault)")
		}

		isFault, _ := eventsourcing.IsConcurrencyFault(secondCommitErr)
		if !isFault {
			return fmt.Errorf("Expected concurrency fault, got: %v", secondCommitErr)
		}
		return nil
	})
}

// CheckWritePastEnd validates you must have an event at N to write at N+1 positions.
func CheckWritePastEnd(t *testing.T, provider StoreProvider) {
	execute(t, provider, func(store eventsourcing.EventStore) error {
		dummyKey := getDummyKey()
		writer := &fakeStoreWriter{
			key:               dummyKey,
			eventRegistry:     GetTestRegistry(),
			uncommittedOrigin: 3,
			uncommittedEvents: []eventsourcing.Event{
				IncrementEvent{},
			},
		}

		errStore := store.CommitEvents(writer)
		if errStore == nil {
			return fmt.Errorf("Expected an error when writing past end, got none")
		}

		return nil
	})
}

// CheckWriteReadWriteRead checks that we can perform a looping read/write cycle
// where we keep appending events to an aggregate.
func CheckWriteReadWriteRead(t *testing.T, provider StoreProvider) {
	execute(t, provider, func(store eventsourcing.EventStore) error {
		agg := SimpleAggregate{}
		dummyKey := getDummyKey()
		agg.Initialize(dummyKey, GetTestRegistry(), store)
		agg.Refresh()
		agg.ApplyEvent(InitializeEvent{
			TargetValue: 50,
		})
		errInitial := agg.Commit()
		if errInitial != nil {
			return errInitial
		}

		for x := 0; x < 250; x++ {
			agg.ApplyEvent(IncrementEvent{
				IncrementBy: 1,
			})
			errCommit := agg.Commit()
			if errCommit != nil {
				return errCommit
			}

			errRefresh := agg.Refresh()
			if errRefresh != nil {
				return errRefresh
			}
		}

		return nil
	})
}

// CheckDirtyRefresh checks that we can't refresh a dirty aggregate from the store
func CheckDirtyRefresh(t *testing.T, provider StoreProvider) {
	execute(t, provider, func(store eventsourcing.EventStore) error {
		agg := SimpleAggregate{}
		dummyKey := getDummyKey()
		agg.Initialize(dummyKey, GetTestRegistry(), store)
		agg.Refresh()
		agg.ApplyEvent(InitializeEvent{
			TargetValue: 50,
		})
		errInitial := agg.Commit()
		if errInitial != nil {
			return errInitial
		}

		for x := 0; x < 10; x++ {
			agg.ApplyEvent(IncrementEvent{
				IncrementBy: 1,
			})
		}

		errRefresh := agg.Refresh()
		if errRefresh == nil {
			return fmt.Errorf("Should not have been able to refresh: aggregate was unclean")
		}

		return nil
	})
}

// MeasureIndividualCommits runs a test that measures how fast we can sequentially
// append to an aggregate.
func MeasureIndividualCommits(b *testing.B, provider StoreProvider) {
	instance := SimpleAggregate{}
	dummyKey := getDummyKey()
	executeBench(b, provider, func(store eventsourcing.EventStore) error {
		instance.Initialize(dummyKey, GetTestRegistry(), store)

		for i := 0; i < b.N; i++ {
			instance.ApplyEvent(IncrementEvent{
				IncrementBy: 1,
			})
			err := instance.Commit()
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// MeasureBulkInsertAndReload measures a bulk insert of 1000 events and then reloads
// the aggregate.
func MeasureBulkInsertAndReload(b *testing.B, provider StoreProvider) {
	executeBench(b, provider, func(store eventsourcing.EventStore) error {
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("Aggregate-%v", i)
			instance := SimpleAggregate{}
			instance.Initialize(key, GetTestRegistry(), store)
			instance.Refresh()
			for x := 0; x < 1000; x++ {
				instance.ApplyEvent(IncrementEvent{
					IncrementBy: 1,
				})
			}
			instance.Commit()

			// Reload
			reload := SimpleAggregate{}
			reload.Initialize(key, GetTestRegistry(), store)
			reload.Refresh()
			assert.Equal(b, 1000, instance.CurrentCount)
		}
		return nil
	})
}

type fakeStoreWriter struct {
	key               string
	eventRegistry     eventsourcing.EventRegistry
	uncommittedOrigin int64
	uncommittedEvents []eventsourcing.Event
}

// GetKey gets the key of the aggregate being stored
func (instance *fakeStoreWriter) GetKey() string {
	return instance.key
}

// GetEventRegistry gets the event registry for the events being stored
func (instance *fakeStoreWriter) GetEventRegistry() eventsourcing.EventRegistry {
	return instance.eventRegistry
}

// GetUncommittedEvents gets the events that are uncommittedEvents for this aggregate.
func (instance *fakeStoreWriter) GetUncommittedEvents() (int64, []eventsourcing.Event) {
	return instance.uncommittedOrigin, instance.uncommittedEvents
}

// IsDirty is reture if there's an uncommitted event
func (instance *fakeStoreWriter) IsDirty() bool {
	return len(instance.uncommittedEvents) > 0
}

// SequenceNumber fetches the current sequence number
func (instance *fakeStoreWriter) SequenceNumber() int64 {
	return instance.uncommittedOrigin + int64(len(instance.uncommittedEvents))
}

// GetState returns an empty state map
func (instance *fakeStoreWriter) GetState() interface{} {
	return map[string]interface{}{}
}
