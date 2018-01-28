package test

import "github.com/go-gadgets/eventsourcing"

var counterRegistry eventsourcing.EventRegistry

func init() {
	counterRegistry = eventsourcing.NewStandardEventRegistry("Testing")
	counterRegistry.RegisterEvent(InitializeEvent{})
	counterRegistry.RegisterEvent(IncrementEvent{})
}

// GetTestRegistry returns the test registry for the library.
func GetTestRegistry() eventsourcing.EventRegistry {
	return counterRegistry
}

// SimpleAggregate is a simple aggregate that counts up or
// down.
type SimpleAggregate struct {
	eventsourcing.AggregateBase
	CurrentCount int `json:"current_count"`
	TargetValue  int `json:"target_value"`
}

// Initialize the aggregate
func (agg *SimpleAggregate) Initialize(key string, registry eventsourcing.EventRegistry, store eventsourcing.EventStore) {
	agg.AggregateBase.Initialize(key, registry, store, func() interface{} { return agg })
	agg.AggregateBase.AutomaticWireup(agg)
}

// ReplayInitializeEvent applies an InitializeEvent to the model.
func (agg *SimpleAggregate) ReplayInitializeEvent(event InitializeEvent) {
	agg.TargetValue = event.TargetValue
}

// ReplayIncrementEvent applies an IncrementEvent to the model.
func (agg *SimpleAggregate) ReplayIncrementEvent(event IncrementEvent) {
	agg.CurrentCount += event.IncrementBy
}

// ReplayEventWithInvalidReturnMapping has a return value, and should not
// be wired up.
func (agg *SimpleAggregate) ReplayEventWithInvalidReturnMapping(event EventWithInvalidReturnMapping) int {
	agg.CurrentCount += event.IncrementBy
	return agg.CurrentCount
}

// ReplayEventWithTooManyArgumentsMapping has an extra parameter, and should not trigger
// an increment event.
func (agg *SimpleAggregate) ReplayEventWithTooManyArgumentsMapping(event EventWithInvalidReturnMapping, extraParameter int) {
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
// supported by SimpleAggregate. What should happen here is that the sequence
// jumps even though it's not wired up.
type UnknownEventTypeExample struct {
}
