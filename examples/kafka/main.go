package main

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/Shopify/sarama"
	"github.com/gin-gonic/gin"
	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/distribution/kafka"
	"github.com/go-gadgets/eventsourcing/stores/middleware/logging"
	"github.com/go-gadgets/eventsourcing/stores/middleware/memorysnap"
	"github.com/go-gadgets/eventsourcing/stores/middleware/mongosnap"
	"github.com/go-gadgets/eventsourcing/stores/middleware/publish"
	"github.com/go-gadgets/eventsourcing/stores/mongo"
	"github.com/sirupsen/logrus"
)

var (
	dataHost = "mongodb://localhost:27017"
	database = "eventsourcingPubSubExample"
	broker   = "localhost:9092"
	topic    = "events"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	logrus.SetLevel(logrus.DebugLevel)

	switch os.Args[1] {
	case "consumer":
		group := os.Args[2]
		runClient(group)
		break
	case "publisher":
		runPublisher()
		break
	default:
		fmt.Println("Please specify argument (consumer/publisher)")
		break
	}
}

// runClient runs a client that listens for messages from kafka
func runClient(group string) {
	// trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	brokers := []string{broker}
	consumer, errConsumer := kafka.CreateConsumer(brokers, topic, group, sarama.OffsetOldest)
	if errConsumer != nil {
		panic(errConsumer)
	}
	handler := &Handler{}
	handler.Initialize(registry, handler)
	consumer.AddHandler(handler)

	errStart := consumer.Start()
	if errStart != nil {
		panic(errStart)
	}
	defer consumer.Stop()

	for {
		select {
		case <-signals:
			return
		}
	}
}

// runPublisher runs the publishing side, writing an event every second
func runPublisher() {
	// Initialze the event store
	mongoStore, errStore := mongo.NewStore(mongo.Endpoint{
		DialURL:        dataHost,
		DatabaseName:   database,
		CollectionName: "Counters",
	})
	if errStore != nil {
		panic(errStore)
	}

	store := eventsourcing.NewMiddlewareWrapper(mongoStore)

	// Post-publish to Kafka
	pub, errPublisher := kafka.CreatePublisher([]string{broker}, topic, registry)
	if errPublisher != nil {
		panic(errPublisher)
	}
	store.Use(publish.Create(pub))

	// Snapshotting to MongoDB
	mongoSnap, errSnap := mongosnap.Create(mongosnap.Parameters{
		SnapInterval: 10,
	}, mongosnap.Endpoint{
		DialURL:        dataHost,
		DatabaseName:   database,
		CollectionName: "Counters-Snapshot",
	})
	if errSnap != nil {
		panic(errSnap)
	}
	store.Use(mongoSnap())

	// Create a lazy in-memory snapshot
	store.Use(memorysnap.Create(memorysnap.Parameters{
		Lazy:         true,
		SnapInterval: 1,
	}))
	// Logging
	store.Use(logging.Create())

	// Just publish every second to Kafka
	for {
		errCommand := eventsourcing.Retry(10, func() error {
			name := "example-aggregate"
			agg := CounterAggregate{}
			agg.Initialize(name, store, func() interface{} { return &agg })

			err := agg.Handle(IncrementCommand{})
			if err != nil {
				return err
			}

			return nil
		})

		if errCommand != nil {
			logrus.Error(errCommand.Error())
		}

		// Sleep for 1 seconds
		time.Sleep(1 * time.Second)
	}
}

// Handler is the projection Handler for the example project
type Handler struct {
	eventsourcing.EventHandlerBase
}

// HandleIncrementEvent consumes an increment event
func (handler *Handler) HandleIncrementEvent(key string, seq int64, evt IncrementEvent) error {
	fmt.Printf("Handling %v:%v=%v\n", key, seq, evt)
	return nil
}
