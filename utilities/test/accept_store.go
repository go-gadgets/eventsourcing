package test

import (
	"fmt"
	"testing"

	"github.com/go-gadgets/eventsourcing"
	"github.com/stretchr/testify/assert"
)

// A StoreProvider is a structure
type StoreProvider func() (eventsourcing.EventStore, func(), error)

// ProviderFunc is a test function that runs in the context of a provider
type ProviderFunc func(store eventsourcing.EventStore) error

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

	fmt.Println("  >> Write new events")
	CheckWriteReadNew(t, provider)

	fmt.Println("  >> Fail when unmapped event is passed")
	CheckUnmappedEvent(t, provider)

	fmt.Println("  >> Concurrency support")
	CheckConcurrencyValidation(t, provider)

	fmt.Println("  >> Write past EOF for aggregate.")
	CheckWritePastEnd(t, provider)
}

// CheckStartupShutdown checks a store starts up and shuts down cleanly.
func CheckStartupShutdown(t *testing.T, provider StoreProvider) {
	execute(t, provider, func(eventsourcing.EventStore) error { return nil })
}

// CheckWriteReadNew validates we can write and then read-lback events
func CheckWriteReadNew(t *testing.T, provider StoreProvider) {
	execute(t, provider, func(store eventsourcing.EventStore) error {
		instance := SimpleAggregate{}
		instance.Initialize("dummy-key", GetTestRegistry(), store)

		instance.ApplyEvent(InitializeEvent{
			TargetValue: 3,
		})

		errCommit := instance.Commit()
		if errCommit != nil {
			return errCommit
		}

		second := SimpleAggregate{}
		second.Initialize("dummy-key", GetTestRegistry(), store)
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
		instance.Initialize("dummy-key", GetTestRegistry(), store)

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
		firstInstance.Initialize("dummy-key", GetTestRegistry(), store)
		firstInstance.Refresh()
		firstInstance.ApplyEvent(InitializeEvent{
			TargetValue: 3,
		})

		secondInstance := SimpleAggregate{}
		secondInstance.Initialize("dummy-key", GetTestRegistry(), store)
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
		writer := &fakeStoreWriter{
			key:              "dummy-key",
			eventRegistry:    GetTestRegistry(),
			uncomittedOrigin: 3,
			uncomittedEvents: []eventsourcing.Event{
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

// MeasureIndividualCommits runs a test that measures how fast we can sequentially
// append to an aggregate.
func MeasureIndividualCommits(b *testing.B, provider StoreProvider) {
	instance := SimpleAggregate{}
	executeBench(b, provider, func(store eventsourcing.EventStore) error {
		instance.Initialize("dummy-key", GetTestRegistry(), store)

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
	key              string
	eventRegistry    eventsourcing.EventRegistry
	uncomittedOrigin int64
	uncomittedEvents []eventsourcing.Event
}

// GetKey gets the key of the aggregate being stored
func (instance *fakeStoreWriter) GetKey() string {
	return instance.key
}

// GetEventRegistry gets the event registry for the events being stored
func (instance *fakeStoreWriter) GetEventRegistry() eventsourcing.EventRegistry {
	return instance.eventRegistry
}

// GetUncomittedEvents gets the events that are uncomittedEvents for this aggregate.
func (instance *fakeStoreWriter) GetUncomittedEvents() (int64, []eventsourcing.Event) {
	return instance.uncomittedOrigin, instance.uncomittedEvents
}

// IsDirty is reture if there's an uncommitted event
func (instance *fakeStoreWriter) IsDirty() bool {
	return len(instance.uncomittedEvents) > 0
}

// SequenceNumber fetches the current sequence number
func (instance *fakeStoreWriter) SequenceNumber() int64 {
	return instance.uncomittedOrigin + int64(len(instance.uncomittedEvents))
}

// GetState returns an empty state map
func (instance *fakeStoreWriter) GetState() interface{} {
	return map[string]interface{}{}
}
