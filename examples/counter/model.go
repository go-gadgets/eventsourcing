package main

import (
	"github.com/go-gadgets/eventsourcing"
)

var registry eventsourcing.EventRegistry

func init() {
	registry = eventsourcing.NewStandardEventRegistry()
	registry.RegisterEvent(IncrementEvent{})
}

// CounterAggregate counts the number of times it's been incremented.
type CounterAggregate struct {
	eventsourcing.AggregateBase `json:"-" bson:"-"`
	Count                       int `json:"count" bson:"count"`
}

// IncrementEvent is an event that moves the counter up.
type IncrementEvent struct {
}

// Initialize the aggregate
func (agg *CounterAggregate) Initialize(key string, store eventsourcing.EventStore, state eventsourcing.StateFetchFunc) {
	agg.AggregateBase.Initialize(key, registry, store, state)
	agg.AutomaticWireup(agg)
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
