package eventsourcing

import (
	"reflect"
	"strings"

	"github.com/go-gadgets/eventsourcing/utilities/mapping"
	"github.com/mitchellh/mapstructure"
)

const (
	// EventHandleMethodPrefix is the prefix for handler auto-wireup methods
	EventHandleMethodPrefix = "Handle"
)

// EventHandlerBase is a common base type for an event handler that takes events
// from a publishing source and handles them.
type EventHandlerBase struct {
	eventConsumers map[EventType]consumerFunc // event consumer methods
	registry       EventRegistry              // Registry for summoning events
}

// Initialize the EventHandlerBase
func (base *EventHandlerBase) Initialize(registry EventRegistry, self interface{}) {
	base.registry = registry
	base.AutomaticWireup(self)
}

// AutomaticWireup performs automatic detection of consumer methods
func (base *EventHandlerBase) AutomaticWireup(subject interface{}) {
	base.eventConsumers = buildConsumeMappings(subject)
}

// Handle processes an event
func (base *EventHandlerBase) Handle(event PublishedEvent) error {
	// If we've got a consumer
	call, found := base.eventConsumers[event.Type]
	if !found {
		return nil
	}

	summoned := base.registry.CreateEvent(event.Type)
	config := &mapstructure.DecoderConfig{
		DecodeHook:       mapping.MapTimeFromJSON,
		TagName:          "json",
		Result:           summoned,
		WeaklyTypedInput: true,
	}
	decoder, errDecoder := mapstructure.NewDecoder(config)
	if errDecoder != nil {
		return errDecoder
	}

	errDecode := decoder.Decode(event.Data)
	if errDecode != nil {
		return errDecode
	}

	return call(event.Key, event.Sequence, summoned)
}

// consumerFunc is a function that consumes an event from a distribution bus.
type consumerFunc func(key string, seq int64, evt Event) error

// buildConsumeMappings builds a set of event replay mappings for a type that has
// methods of a suitable interface. This allows wireup-by-convention for the base
// aggregate type.
func buildConsumeMappings(subject interface{}) map[EventType]consumerFunc {
	eventConsumers := make(map[EventType]consumerFunc)
	subjectType := reflect.TypeOf(subject)
	totalMethods := subjectType.NumMethod()
	for methodIndex := 0; methodIndex < totalMethods; methodIndex++ {
		candidate := subjectType.Method(methodIndex)

		// Skip methods without prefix
		if !strings.HasPrefix(candidate.Name, EventHandleMethodPrefix) {
			continue
		}

		// Method should have two arguments, no outputs
		if candidate.Type.NumIn() != 4 || candidate.Type.NumOut() != 1 {
			continue
		}

		handler := func(key string, seq int64, event Event) error {
			response := candidate.Func.Call([]reflect.Value{
				reflect.ValueOf(subject),
				reflect.ValueOf(key),
				reflect.ValueOf(seq),
				reflect.ValueOf(event).Elem(),
			})

			if len(response) > 0 && !response[0].IsNil() {
				err := response[0].Interface().(error)
				return err
			}

			return nil
		}

		// The event is the 4th element (index 3)
		eventType := candidate.Type.In(3)
		eventTypeName := EventType(NormalizeTypeName(eventType.String()))
		eventConsumers[eventTypeName] = handler
	}

	return eventConsumers
}
