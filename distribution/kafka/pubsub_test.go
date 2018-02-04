package kafka

import (
	"fmt"

	"testing"

	"github.com/go-gadgets/eventsourcing"
	uuid "github.com/satori/go.uuid"
)

const (
	testDomain = "TestDomain"
	testHost   = "localhost:9092"
	testTopic  = "testing"
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
	clusterHosts := []string{testHost}

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

// BenchmarkKafkaPublish tests how fast we can run the publisher
func BenchmarkIndividualCommmits(b *testing.B) {
	clusterHosts := []string{testHost}

	// Create a publisher
	pub, errSetup := CreatePublisher(clusterHosts, testTopic, testRegistry)
	if errSetup != nil {
		b.Error(errSetup)
		return
	}

	testKey := fmt.Sprintf("%s", uuid.NewV4())
	count := 0
	for i := 0; i < b.N; i++ {
		count++
		errPublished := pub.Publish(testKey, int64(count), TickEvent{Numeral: 1234})
		if errPublished != nil {
			b.Error(errPublished)
			return
		}
	}
}
