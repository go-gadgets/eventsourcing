package main

import (
	"github.com/go-gadgets/eventsourcing"
)

var registry eventsourcing.EventRegistry

func init() {
	registry = eventsourcing.NewStandardEventRegistry("DemoModel")
	registry.RegisterEvent(IncrementEvent{})
}

// CounterAggregate counts the number of times it's been incremented.
type CounterAggregate struct {
	eventsourcing.AggregateBase `json:"-"`
	Count                       int `json:"count"`
}

// IncrementCommand is a command to increment an aggregates value
type IncrementCommand struct {
}

// IncrementEvent is an event that moves the counter up.
type IncrementEvent struct {
}

// Initialize the aggregate
func (agg *CounterAggregate) Initialize(key string, store eventsourcing.EventStore, state eventsourcing.StateFetchFunc) {
	agg.AggregateBase.Initialize(key, registry, store, state)
	agg.AutomaticWireup(agg)
}

// HandleIncrementCommand handles an increment command from the bus.
func (agg *CounterAggregate) HandleIncrementCommand(command IncrementCommand) ([]eventsourcing.Event, error) {
	// Raise the events
	return []eventsourcing.Event{
		IncrementEvent{},
	}, nil
}

// Increment increases the counter value.
func (agg *CounterAggregate) Increment() {
	// Do any domain logic here.

	// Apply the events
	agg.ApplyEvent(IncrementEvent{})
}

// ReplayIncrementEvent updates the counter by adding one.
func (agg *CounterAggregate) ReplayIncrementEvent(event IncrementEvent) {
	agg.Count++
}
