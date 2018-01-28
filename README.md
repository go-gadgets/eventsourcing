# eventsourcing

[![Build Status](https://travis-ci.org/go-gadgets/eventsourcing.svg?branch=master)](https://travis-ci.org/go-gadgets/eventsourcing) 
[![GoDoc](https://godoc.org/github.com/go-gadgets/eventsourcing?status.svg)](https://godoc.org/github.com/go-gadgets/eventsourcing) 
[![codecov](https://codecov.io/gh/go-gadgets/eventsourcing/branch/master/graph/badge.svg)](https://codecov.io/gh/go-gadgets/eventsourcing)  [![Go Report Card](https://goreportcard.com/badge/github.com/go-gadgets/eventsourcing)](https://goreportcard.com/report/github.com/go-gadgets/eventsourcing) 
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT) 
[![stability-experimental](https://img.shields.io/badge/stability-experimental-orange.svg)](https://github.com/emersion/stability-badges#experimental)

A framework for implementing the event-sourcing pattern easily in Go.

## Installation
To install this package, please use [gopkg.in](https://gopkg.in/go-gadgets/eventsourcing.v0) instead of Github:

```
  go get gopkg.in/go-gadgets/eventsourcing.v0
```

## Features
The features of this framework are:

 - Low-ceremony:
   - The counter-example is less than 150 lines of code, including snapshot support, Mongo persistence and a web-server API.
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

// HandleIncrementCommand handles an increment command from the bus.
func (agg *CounterAggregate) HandleIncrementCommand(command IncrementCommand) ([]eventsourcing.Event, error) {
  // Insert domain rules here.

	// Raise the events
	return []eventsourcing.Event{
		IncrementEvent{},
	}, nil
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
   - Our aggregate-root type, which leverages the eventsourcing.AggregateBase type for implementing some common functionality.
 - IncrementEvent
   - An event that when replayed, bumps the count up.

Note the use of `AutomaticWireup(agg)`: this is a helper function that scans a type using reflection and configures:

 - Replay Functions
   - Used to recover the present state of an aggregate from the event stream.
   - Use methods that match the `Replay<EventTypeName>(event <EventTypeName>)` method signature.
 - Command Handlers
   - Used to execute commands agains the model.
   - Use methods that match the `Handle<CommandTypeName>(command <CommandTypeName>) ([]eventsourcing.Event, error)` method signature.

To run this code, we can leverage a memory based store:

```
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/go-gadgets/eventsourcing/stores/in-memory"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	store := inmemory.NewStore()

	r := gin.Default()
	r.POST("/:name/increment", func(c *gin.Context) {
		name := c.Param("name")
		agg := CounterAggregate{}
		agg.Initialize(name, store, func() interface{} { return &agg })

		errCommand := agg.Handle(IncrementCommand{})

		if errCommand != nil {
			c.JSON(500, errCommand.Error())
			return
		}

		// Show the count
		c.JSON(200, gin.H{
			"count": agg.Count,
		})

	})
  
  r.Run() // Listen and serve on 0.0.0.0:8080
}
```

