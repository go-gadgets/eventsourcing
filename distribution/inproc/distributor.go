package inproc

import (
	"fmt"

	"github.com/go-gadgets/eventsourcing"
)

// Distributor is the interface for our in-process distributor that
// acts as both publisher and consumer.
type Distributor interface {
	eventsourcing.EventPublisher
	eventsourcing.EventConsumer
}

// distributor is an in-process event distributor that propegates events
// post-store, acting as both a Consumer and Publisher API instance.
type distributor struct {
	enabled  bool                         // Enabled?
	handlers []eventsourcing.EventHandler // Event handlers
	registry eventsourcing.EventRegistry  // Event registry
}

// Create an instance of the Distributor interface
func Create(registry eventsourcing.EventRegistry) Distributor {
	return &distributor{
		handlers: make([]eventsourcing.EventHandler, 0),
		registry: registry,
	}
}

// AddHandler appends a new handler to the set of handlers for this consumer
func (distributor *distributor) AddHandler(handler eventsourcing.EventHandler) {
	distributor.handlers = append(distributor.handlers, handler)
}

// Start handling the events from the consumer
func (distributor *distributor) Start() error {
	distributor.enabled = true
	return nil
}

// Stop handling events from the consumer
func (distributor *distributor) Stop() error {
	distributor.enabled = false
	return nil
}

// Publish an event.
func (distributor *distributor) Publish(key string, sequence int64, event eventsourcing.Event) error {
	if !distributor.enabled || len(distributor.handlers) == 0 {
		return nil
	}

	eventType, found := distributor.registry.GetEventType(event)
	if !found {
		return fmt.Errorf("Could not find event type: %v", event)
	}

	toPublish := eventsourcing.PublishedEvent{
		Domain:   distributor.registry.Domain(),
		Type:     eventType,
		Key:      key,
		Sequence: sequence,
		Data:     event,
	}

	for _, handler := range distributor.handlers {
		errHandle := handler.Handle(toPublish)
		if errHandle != nil {
			return errHandle
		}
	}

	return nil
}
