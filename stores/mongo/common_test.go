package mongo

import "github.com/go-gadgets/eventsourcing"

var counterRegistry eventsourcing.EventRegistry

func init() {
	counterRegistry = eventsourcing.NewStandardEventRegistry()
	counterRegistry.RegisterEvent(InitializeEvent{})
	counterRegistry.RegisterEvent(IncrementEvent{})
}

// TestOnlyAggregate is a simple aggregate that counts up or
// down.
type TestOnlyAggregate struct {
	eventsourcing.AggregateBase
	CurrentCount int `json:"current_count"`
	TargetValue  int `json:"target_value"`
}

// Initialize the aggregate
func (agg *TestOnlyAggregate) Initialize(key string, registry eventsourcing.EventRegistry, store eventsourcing.EventStore) {
	agg.AggregateBase.Initialize(key, registry, store, func() interface{} { return agg })
	agg.AggregateBase.AutomaticWireup(agg)
}

func (agg *TestOnlyAggregate) ReplayInitializeEvent(event InitializeEvent) {
	agg.TargetValue = event.TargetValue
}

func (agg *TestOnlyAggregate) ReplayIncrementEvent(event IncrementEvent) {
	agg.CurrentCount += event.IncrementBy
}

// ReplayEventWithInvalidReturnMapping has a return value, and should not
// be wired up.
func (agg *TestOnlyAggregate) ReplayEventWithInvalidReturnMapping(event EventWithInvalidReturnMapping) int {
	agg.CurrentCount += event.IncrementBy
	return agg.CurrentCount
}

// ReplayEventWithTooManyArgumentsMapping has an extra parameter, and should not trigger
// an increment event.
func (agg *TestOnlyAggregate) ReplayEventWithTooManyArgumentsMapping(event EventWithInvalidReturnMapping, extraParameter int) {
	agg.CurrentCount += event.IncrementBy
}

// InitializeEvent is an event that initializes the current state
// of an event.
type InitializeEvent struct {
	// TargetValue is the value the counter will count towards.
	TargetValue int `json:"target_value"`
}

// IncrementEvent represents an event that increments the model value
type IncrementEvent struct {
	IncrementBy int `json:"increment_by"`
}

// EventWithInvalidReturnMapping is an event that does not have a good
// mapping - it has a reutrn value.
type EventWithInvalidReturnMapping struct {
	IncrementBy int `json:"increment_by"`
}

// EventWithTooManyArgumentsMapping is an event that does not have a good
// mapping - it has too many arguments to the replay method.
type EventWithTooManyArgumentsMapping struct {
	IncrementBy int `json:"increment_by"`
}

// UnknownEventTypeExample is an event that is just made out of the ether, and is not
// supported by TestOnlyAggregate. What should happen here is that the sequence
// jumps even though it's not wired up.
type UnknownEventTypeExample struct {
}
