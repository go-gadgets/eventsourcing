package kafka

import (
	"fmt"
	"time"

	"testing"

	"math/rand"

	"github.com/Shopify/sarama"
	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/utilities/test"
	uuid "github.com/satori/go.uuid"
)

const (
	testHost  = "localhost:9092"
	testTopic = "testing"
)

// TestKafkaPublishing tests kafka-publishing is operating as expected
// using the CreatePublisher() API.
func TestKafkaPublishing(t *testing.T) {
	clusterHosts := []string{testHost}

	// Create a publisher
	pub, errSetup := CreatePublisher(clusterHosts, testTopic, test.GetTestRegistry())
	if errSetup != nil {
		t.Error(errSetup)
		return
	}

	errPublished := pub.Publish("dummy-key", 1, test.IncrementEvent{IncrementBy: 1234})
	if errPublished != nil {
		t.Error(errPublished)
		return
	}
}

// TestKafkaConsumption runs a consumer and checks that events get
// passed to it within the specified timeout.
func TestKafkaConsumption(t *testing.T) {
	// Create a publisher
	clusterHosts := []string{testHost}
	pub, errSetup := CreatePublisher(clusterHosts, testTopic, test.GetTestRegistry())
	if errSetup != nil {
		t.Error(errSetup)
		return
	}

	// Test values
	target := rand.Int()
	testKey := fmt.Sprintf("%s", uuid.NewV4())
	outcomes := make(chan bool, 1)

	go func() {
		time.Sleep(time.Second * 10)
		outcomes <- false
	}()

	// Create a group consumer
	group := fmt.Sprintf("%v", uuid.NewV4())
	consumer, errConsumer := CreateConsumer(clusterHosts, testTopic, group, sarama.OffsetNewest)
	if errConsumer != nil {
		panic(errConsumer)
	}
	handler := &testHandler{
		subject: testKey,
		target:  target,
		success: outcomes,
	}
	handler.Initialize(test.GetTestRegistry(), handler)
	consumer.AddHandler(handler)
	errStart := consumer.Start()
	if errStart != nil {
		panic(errStart)
	}
	defer consumer.Stop()
	time.Sleep(time.Second * 5)
	msg := test.IncrementEvent{IncrementBy: target}
	errPublished := pub.Publish(testKey, 1, msg)
	if errPublished != nil {
		t.Error(errPublished)
		return
	}

	select {
	case result := <-outcomes:
		if !result {
			t.Errorf("Timeout expired waiting for consumer")
		}
	}
}

// BenchmarkSerialKafkaPublish tests how fast we can run the publisher
// in serial, pumping a single message a time.
func BenchmarkSerialKafkaPublish(b *testing.B) {
	clusterHosts := []string{testHost}

	// Create a publisher
	pub, errSetup := CreatePublisher(clusterHosts, testTopic, test.GetTestRegistry())
	if errSetup != nil {
		b.Error(errSetup)
		return
	}

	testKey := fmt.Sprintf("%s", uuid.NewV4())
	count := 0
	for i := 0; i < b.N; i++ {
		count++
		errPublished := pub.Publish(testKey, int64(count), test.IncrementEvent{IncrementBy: 1234})
		if errPublished != nil {
			b.Error(errPublished)
			return
		}
	}

}

// testHandler watches for a specified value to be sent.
type testHandler struct {
	subject string
	success chan bool
	target  int
	eventsourcing.EventHandlerBase
}

// HandleIncrementEvent consumes an increment event
func (handler *testHandler) HandleIncrementEvent(key string, seq int64, evt test.IncrementEvent) error {
	if key == handler.subject && evt.IncrementBy == handler.target {
		handler.success <- true
	}
	return nil
}
