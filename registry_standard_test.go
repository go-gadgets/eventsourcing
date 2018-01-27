package eventsourcing

import "testing"
import "github.com/stretchr/testify/assert"

// Notes: The remainder of the testing of this registry is more than amply covered by other tests, for now.

// TestRegistryStandardCreateUnmappedEvent checks that when an event is undefined, it gets summoned as
// a map[string]interface{}
func TestRegistryStandardCreateUnmappedEvent(t *testing.T) {
	registry := NewStandardEventRegistry()
	instance := registry.CreateEvent(EventType("Does-Not-Exist"))

	_, ok := instance.(map[string]interface{})
	assert.True(t, ok, "The instance should a map[string]interface{}")
}
