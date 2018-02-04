package kafka

import (
	"testing"

	"github.com/go-gadgets/eventsourcing"
)

const (
	testDomain = "TestDomain"
)

var testRegistry eventsourcing.EventRegistry

func init() {
	testRegistry = eventsourcing.NewStandardEventRegistry(testDomain)
	testRegistry.RegisterEvent(TickEvent{})
}

// TickEvent represents a tick-event in our tick-tock test model
type TickEvent struct {
	Numeral int `json:"numeral"`
}

// TestKafkaPublishing tests kafka-publishing is operating as expected
// using the CreatePublisher() API.
func TestKafkaPublishing(t *testing.T) {
	clusterHosts := []string{"localhost:9092"}
	testTopic := "testing"

	// Create a publisher
	pub, errSetup := CreatePublisher(clusterHosts, testTopic, testRegistry)
	if errSetup != nil {
		t.Error(errSetup)
		return
	}

	errPublished := pub.Publish("dummy-key", 1, TickEvent{Numeral: 1234})
	if errPublished != nil {
		t.Error(errPublished)
		return
	}
}
