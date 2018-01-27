# eventsourcing

[![Build Status](https://travis-ci.org/go-gadgets/eventsourcing.svg?branch=master)](https://travis-ci.org/go-gadgets/eventsourcing)
[![GoDoc](https://godoc.org/github.com/go-gadgets/eventsourcing?status.svg)](https://godoc.org/github.com/go-gadgets/eventsourcing)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-gadgets/eventsourcing)](https://goreportcard.com/report/github.com/go-gadgets/eventsourcing)

An opinionated event-sourcing framework, written in Go.

## Features
The features of this framework are:

 - Low-ceremony:
   - The counter-example is less than 100 lines of code, including snapshot support, Mongo persistence and a web-server API.
 - Pluggable event-store engines:
   - MongoDB
   - InMemory
   - NullStore
 - Snapshot optimisation
   - Allows aggregates to be snapshotted at intervals.
 - Quick-Start helper types:
   - The AggregateBase type allows for fast creation of aggregates and uses reflection in order to wire-up event replay methods.
  

## What is Event-Sourcing?
Event-Sourcing is an architectural pattern in which the state of an entity in your application is modelled as a series of events, mutating the state. For example, we may store the history of a bank account:

| Aggregate Key | Sequence | Event Data |
|---------------|----------|------------|
| 123456 | 1 | Account Created |
| 123456 | 2 | Deposit ($50) |
| 123456 | 3 | Withdrawl ($25) |

If we now had to consider a bank account withdrawl, we would:
 
  - Create an empty (Sequence-0) bank account entity.
  - Fetch events from the event-store and apply them to the model
     - Repeat until we have completely rehydrated the state.
  - Check the 'Balance' property of the model
  - Write the new event.

At any given point in time, we can track and identify the state of an entity. It's also possible to understand exactly the sequence of events that led to an outcome being selected.

## Creating Your Aggregate-Root
An aggregate (root) is an entity that's defined by the series of events that happen to it. In this simple example (found under `/examples/counter` within this repository), we'll look at an aggregate that counts the times it's incremented:

````
var registry eventsourcing.EventRegistry

func init() {
	registry = eventsourcing.NewStandardEventRegistry()
	registry.RegisterEvent(IncrementEvent{})
}

type CounterAggregate struct {
	eventsourcing.AggregateBase 
	Count int 
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
````

In this example we have:

 - A registry
   - A registry identifies the types of events that apply to a model, acting as a helper for mapping stored events back to real types.
 - CounterAggregate
   - Our aggregate-root type, which leverages the goincidence.AggregateBase type for implementing some common functionality.
 - IncrementEvent
   - An event that when replayed, bumps the count up.

Note the use of `AutomaticWireup(agg)`: this is a helper function that scans a type using reflection and configures event replays to use methods that match the `Replay<EventTypeName>(event <EventTypeName>)` method signature.

To run this code, we can leverage a memory based store:

```
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/go-gadgets/eventsourcing/stores/in-memory"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	store := inmemory.NewInMemoryStore()

	r := gin.Default()
	r.GET("/:name/increment", func(c *gin.Context) {
		name := c.Param("name")

		agg := CounterAggregate{}
		agg.Initialize(name, store, func() interface{} { return &agg })

		errRun := agg.Run(func() error {
			agg.Increment()
			return nil
		})

		if errRun != nil {
			c.JSON(500, errRun.Error())
			return
		}

		// Show the count
		c.JSON(200, gin.H{
			"count": agg.Count,
		})
	})
	r.Run() // listen and serve on 0.0.0.0:8080
}
```

