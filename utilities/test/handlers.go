package test

import "github.com/go-gadgets/eventsourcing"

// LoggingHandler is a handler that tracks events into a collection
// so we can see what's been sent to the handler
type LoggingHandler struct {
	Events []eventsourcing.PublishedEvent // Published events
}

// CreateLoggingHandler creates a handler that can process events
func CreateLoggingHandler() LoggingHandler {
	return LoggingHandler{
		Events: make([]eventsourcing.PublishedEvent, 0),
	}
}

// Handle an event
func (logger *LoggingHandler) Handle(event eventsourcing.PublishedEvent) error {
	logger.Events = append(logger.Events, event)
	return nil
}
