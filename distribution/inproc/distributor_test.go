package inproc

import (
	"testing"

	"github.com/go-gadgets/eventsourcing/utilities/test"
	"github.com/stretchr/testify/assert"
)

// TestBasicHandling runs the basic handling checks to make sure events dispatch
func TestBasicHandling(t *testing.T) {
	// Arrange
	dist := Create(test.GetTestRegistry())
	handler := test.CreateLoggingHandler()
	dist.AddHandler(&handler)
	dist.Start()
	defer dist.Stop()

	// Act
	dist.Publish("dummy", 1, test.IncrementEvent{IncrementBy: 1234})

	// Assert
	assert.Equal(t, 1, len(handler.Events))
}

// TestNoPublishUntilStart checks publishes dont't run unless the Start is called
func TestNoPublishUntilStart(t *testing.T) {
	// Arrange
	dist := Create(test.GetTestRegistry())
	handler := test.CreateLoggingHandler()
	dist.AddHandler(&handler)

	// Act
	dist.Publish("dummy", 1, test.IncrementEvent{IncrementBy: 1234})

	// Assert
	assert.Equal(t, 0, len(handler.Events))

	// Arrrange
	dist.Start()
	dist.Stop()

	// Act
	dist.Publish("dummy", 1, test.IncrementEvent{IncrementBy: 1234})

	// Assert
	assert.Equal(t, 0, len(handler.Events))
}

// TestPublishInvalidEvent checks we don't publish invalid events
func TestPublishInvalidEvent(t *testing.T) {
	// Arrange
	dist := Create(test.GetTestRegistry())
	handler := test.CreateLoggingHandler()
	dist.AddHandler(&handler)
	dist.Start()
	defer dist.Stop()

	// Act
	errPublish := dist.Publish("dummy", 1, test.UnknownEventTypeExample{})

	// Assert
	assert.Equal(t, 0, len(handler.Events))
	assert.NotNil(t, errPublish)
}
