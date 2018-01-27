package eventsourcing

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBaseAggregateEventApplication checks that we are able to apply and
// mutate state in basic ways for a simple aggregate.
func TestBaseAggregateEventApplication(t *testing.T) {
	instance := &SimpleAggregate{}
	store := NewNullStore()
	instance.Initialize("dummy-key", counterRegistry, store)
	instance.Refresh()

	assert.Equal(t, int64(0), instance.SequenceNumber(), "The aggregate sequence number should be 0")
	assert.Equal(t, 0, instance.TargetValue, "The aggregate target value should be 0 / default")

	instance.ApplyEvent(InitializeEvent{
		TargetValue: 3,
	})

	assert.Equal(t, int64(1), instance.SequenceNumber(), "The aggregate sequence number should be 1")
	assert.Equal(t, 3, instance.TargetValue, "The aggregate target value should be 3")
}

// TestBaseAggregateRun checks we can use the run method.
func TestBaseAggregateRun(t *testing.T) {
	instance := &SimpleAggregate{}
	store := NewNullStore()
	instance.Initialize("dummy-key", counterRegistry, store)
	instance.Run(func() error {
		instance.Init(3)
		return nil
	})

	assert.Equal(t, int64(1), instance.SequenceNumber(), "The aggregate sequence number should be 1")
	assert.Equal(t, 3, instance.TargetValue, "The aggregate target value should be 3")
}

// TestBaseAggregateEventCommit checks that we commit events and clear the state as expected.
func TestBaseAggregateEventCommit(t *testing.T) {
	instance := &SimpleAggregate{}
	store := NewNullStore()
	instance.Initialize("dummy-key", counterRegistry, store)
	instance.Refresh()
	assert.False(t, instance.isDirty(), "The aggregate should not be dirty before any events.")

	instance.ApplyEvent(InitializeEvent{
		TargetValue: 3,
	})
	assert.True(t, instance.isDirty(), "The aggregate should be dirty, after applying an event")

	instance.Commit()
	assert.False(t, instance.isDirty(), "The aggregate should not be dirty after comitting events.")
}

// TestBaseAggregateIgnoreUnmappedEvents checks that undefined events are handled by no-op and only
// bumping the aggregate sequence number.
func TestBaseAggregateIgnoreUnmappedEvents(t *testing.T) {
	instance := &SimpleAggregate{}
	store := NewNullStore()
	instance.Initialize("dummy-key", counterRegistry, store)
	instance.Refresh()

	// Apply our unknown event
	instance.ApplyEvent(UnknownEventTypeExample{})

	assert.Equal(t, int64(1), instance.SequenceNumber(), "The aggregate sequence number should be 1")
	assert.Equal(t, 0, instance.TargetValue, "The aggregate target value should be 0")
}

// TestBaseAggregateDefineReplayMethod checks that replay methods work as intended.
func TestBaseAggregateDefineReplayMethod(t *testing.T) {
	instance := &SimpleAggregate{}
	store := NewNullStore()
	instance.Initialize("dummy-key", counterRegistry, store)
	instance.Refresh()

	// Nothing should happen yet. Define the event type in our registry, but it should still not change
	// anything state-wise.
	instance.ApplyEvent(UnknownEventTypeExample{})
	eventType := counterRegistry.RegisterEvent(UnknownEventTypeExample{})
	assert.Equal(t, int64(1), instance.SequenceNumber(), "The aggregate sequence number should be 1")
	assert.Equal(t, 0, instance.TargetValue, "The aggregate target value should be 0")

	instance.DefineReplayMethod(eventType, func(evt interface{}) {
		instance.TargetValue *= 2
	})

	// Set target value
	instance.ApplyEvent(InitializeEvent{
		TargetValue: 3,
	})

	// Apply our unknown event
	instance.ApplyEvent(UnknownEventTypeExample{})

	assert.Equal(t, int64(3), instance.SequenceNumber(), "The aggregate sequence number should be 3")
	assert.Equal(t, 6, instance.TargetValue, "The aggregate target value should be 6")
}

// TestBaseAggregateErrorPropegation tests that the aggregate base type propegates errors
// from stores as expected.
func TestBaseAggregateErrorPropegation(t *testing.T) {
	instance := &SimpleAggregate{}
	store := &errorStore{
		errorToReturn: errors.New("Example error"),
	}
	instance.Initialize("dummy-key", counterRegistry, store)
	errLoad := instance.Refresh()
	assert.Equal(t, store.errorToReturn, errLoad)

	// Apply our unknown event
	instance.ApplyEvent(UnknownEventTypeExample{})

	errSave := instance.Commit()
	assert.Equal(t, store.errorToReturn, errSave)
}

// BenchmarkBaseAggregateWireupSpeed checks how fast an aggregate is initialized
// and goes through the reflection-heavy startup process.
func BenchmarkBaseAggregateWireupSpeed(b *testing.B) {
	store := NewNullStore()
	for i := 0; i < b.N; i++ {
		instance := &SimpleAggregate{}
		instance.Initialize("dummy-key", counterRegistry, store)
	}
}
