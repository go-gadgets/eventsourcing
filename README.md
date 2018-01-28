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
 - Simple structure annotations:
   - Just use the `json:"name"` tag on your aggregates/events to persist fields, without worrying about your underlying storage engine.
   - For Mongo, this required a custom fork of [mgo](https://github.com/globalsign/mgo), found [here](https://github.com/steve-gray/mgo-eventsourcing/).

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

## Questions & Answers

#### Can I call external methods in my Replay functions?
__Don't! Stop!__ Replay methods should act atomically upon the aggregate and _only_ the aggregate - not calling out to anything else that could impact the decision or control flow.
This is a mandatory element for reliable event-sourcing:

 - Events represent something that _has_ happened: you should not make decisions about this.
 - If you rely on externalities, the aggregate state could be different between two refreshes/loads.

You should call any externalities in your _Command_ handling functions, and then once you're satisfied that the model can mutate, you raise the events.

#### When my Aggregate has a long life, operations are slow - Why?
When loading events from the backing stores, _all_ prior events must be loaded and processed in order to 'catch up' and execute the command
versus the latest state. In the case of long-lived aggregates, this can be one hell of a long history and will be accordingly slow. It's recommended that:

 - __Use Snapshotting__
   - All built-in providers support snapshotting, allowing the state of an aggregate to be cached every N events. This allows for faster
	   replay, since only the last N events _at most_ need to be fetched from the backing store.
 - __Spread the Events Among Aggregates__
   - Aggregates should be numerous. This allows for scaling through sharding with most providers for backing storage.
   - If you've got a single large aggregate that's your entire application, something needs to change there.

If you follow these practices, you'll get great performance - and the ability to scale.

#### Why are you using Reflection?
There's a few spots where it's required to use reflection/marshalling of types from generic structures (i.e. Turning BSON/JSON back into a structure or vice-versa). Some other areas which leverage reflection can be avoided if you're prepared to do a little bit of extra leg-work:

 - __AggregateBase__
   - The AggregateBase can be used with the `AutomaticWireup` method to dynamically register event handler methods and command handlers.
	 - If you _dont_ call .AutomaticWireup, you can:
	   - For each event type, call `agg.DefineReplayMethod(eventType, replay func(Event))` to manually define the event type.

In short, if you're keen to avoid those reflecftion calls - you can - but there's a price to pay in terms of code effort - and if you're using a real-world backing store generally it won't be a good trade.

#### Where is the Event-Bus Concept?
This is something that _most_ (possibly _all_) event-sourcing implementations out there do poorly it seems.
The challenges are that document-databases for storing events either have one of two limitations:

 - Maximum document size (i.e. Some frameworks store all events as an Array on a single document, meaning serious history limits)
   - Nothing stops you writing a Mongo-store variant that implements this pattern here.
 - Non-Transactionality of the bus (i.e. Even though it got into MongoDB, did it get published/acted on elsewhere?)

For this reason I've explicitly decided to _not_ deal with the event-bus as a concept in the framework, instead recommending:

 - Use your event-stores native back-end to propegate events to subscribers via an intermediary.
   - __Mongo__ - Tail the oplog on your database into Kafka.
	 - __DynamoDB__ - Enable kinesis streams and attach handlers for distribution to those.
 - Design aggregate events such that one-command triggers one-event in the general case, or that commands are retryable in the case of not
  all events getting published to the store (i.e. Of 10 events, only first 5 got written in a batch)

This ultimately reduces the complexity required in the package, but also means that you'll get a more reliable delivery of events to targets.

#### What About Command-Buses?

Making a generic command-bus is essentially an exercise in reflection-abuse in Go, so instead the library is currently focused on making BYO-bus as easy as possible.
The preferred pattern for this project is that your model gets exposed as a service (i.e. HTTP-ReST or similiar) and then people interact with that, without reference
to the fact that under the hood you are using event-sourcing.

#### How do I do Consistent Reads?
Use the `aggregate.Run((cb) => {})` methods. During the callback the aggregate will be revived to the latest/current state. Be mindful of using this excessively
though and instead bias towards using projections, unless there is a distinct and genuine reason to hit your event-store with the read commands.
