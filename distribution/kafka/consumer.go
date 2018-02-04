package kafka

import (
	"encoding/json"

	cluster "github.com/bsm/sarama-cluster"
	"github.com/go-gadgets/eventsourcing"
	"github.com/sirupsen/logrus"
)

type consumer struct {
	brokers         []string                     // Broker list
	groupID         string                       // Consumer group ID
	topic           string                       // Topic to listen to
	defaultOffset   int64                        // Default offset to listen to (sarama.OffsetOldest/sarama.OffsetNewest)
	closeChannel    chan bool                    // Close signal
	clusterConsumer *cluster.Consumer            // Kafka consumer
	handlers        []eventsourcing.EventHandler // Event handlers
}

// CreateConsumer creates a new consumer of kafka messages.
func CreateConsumer(brokers []string, topic string, groupID string, defaultOffset int64) (eventsourcing.EventConsumer, error) {
	return &consumer{
		brokers:       brokers,
		topic:         topic,
		groupID:       groupID,
		defaultOffset: defaultOffset,
		closeChannel:  make(chan bool, 1),
		handlers:      make([]eventsourcing.EventHandler, 0),
	}, nil
}

// AddHandler appends a new handler to the set of handlers for this consumer
func (consumer *consumer) AddHandler(handler eventsourcing.EventHandler) {
	consumer.handlers = append(consumer.handlers, handler)
}

// Start handling the events from the consumer
func (consumer *consumer) Start() error {
	// Connfiguration for cluster listener
	config := cluster.NewConfig()
	config.Consumer.Return.Errors = true                     // For logging
	config.Consumer.Offsets.Initial = consumer.defaultOffset // Start at right place
	config.Group.Return.Notifications = true                 // For logging

	// Build the cluster listener
	topics := []string{consumer.topic}
	clusterConsumer, err := cluster.NewConsumer(consumer.brokers, consumer.groupID, topics, config)
	if err != nil {
		return err
	}

	consumer.clusterConsumer = clusterConsumer
	go consumer.handleInternal()
	return nil
}

// Stop handling events from the consumer
func (consumer *consumer) Stop() error {
	if consumer.clusterConsumer == nil {
		return nil
	}

	// Close the consumer
	consumer.closeChannel <- true
	consumer.clusterConsumer.Close()
	consumer.clusterConsumer = nil

	return nil
}

// dispatch runs an event through all available handlers
func (consumer *consumer) dispatch(event eventsourcing.PublishedEvent) error {
	for _, handler := range consumer.handlers {
		errHandler := handler.Handle(event)
		if errHandler != nil {
			return errHandler
		}
	}

	return nil
}

// handleInternal runs the kafka consumers internal behaviours.
func (consumer *consumer) handleInternal() {
	instance := consumer.clusterConsumer

	// consume errors
	go func() {
		for err := range instance.Errors() {
			logrus.Error(err)
		}
	}()

	// consume notifications
	go func() {
		for ntf := range instance.Notifications() {
			logrus.Warn(ntf)
		}
	}()

	for {
		select {
		case msg, ok := <-instance.Messages():
			if !ok {
				continue
			}

			// Unmarshal the published event
			event := eventsourcing.PublishedEvent{}
			errUnmarshal := json.Unmarshal(msg.Value, &event)
			if errUnmarshal != nil {
				logrus.Error(errUnmarshal)
				continue
			}

			errConsume := consumer.dispatch(event)
			if errConsume != nil {
				logrus.Error(errConsume)
				continue
			}

			instance.MarkOffset(msg, "")
		case <-consumer.closeChannel:
			logrus.Info("kafka_consumer_closing")
			return
		}
	}
}
