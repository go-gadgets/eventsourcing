package kafka

import (
	"encoding/json"
	"fmt"

	"github.com/Shopify/sarama"
	"github.com/go-gadgets/eventsourcing"
)

// publisher is a structure implementing EventPublisher and storing events into
// a designated Kafka topic.
type publisher struct {
	prod     sarama.SyncProducer         // Producer connection
	topic    string                      // Topic to publish to
	registry eventsourcing.EventRegistry // Registry
}

// CreatePublisher creates a new kafka publisher from a set of hosts, using the default
// publisher settings.
func CreatePublisher(brokers []string, topic string, registry eventsourcing.EventRegistry) (eventsourcing.EventPublisher, error) {
	prod, errProd := sarama.NewSyncProducer(brokers, nil)
	if errProd != nil {
		return nil, errProd
	}

	return CreatePublisherWithProducer(prod, topic, registry)
}

// CreatePublisherWithProducer creates a publisher with a producer that's already been established
// (BYO-instance)
func CreatePublisherWithProducer(prod sarama.SyncProducer, topic string, registry eventsourcing.EventRegistry) (eventsourcing.EventPublisher, error) {
	return &publisher{
		prod:     prod,
		topic:    topic,
		registry: registry,
	}, nil
}

// Publish an event. When the method returns the event should be comitted/guaranteed
// to have been distributed.
func (pub *publisher) Publish(key string, sequence int64, event eventsourcing.Event) error {
	eventType, found := pub.registry.GetEventType(event)
	if !found {
		return fmt.Errorf("Could not find event type: %v", event)
	}

	toPublish := eventsourcing.PublishedEvent{
		Domain:   pub.registry.Domain(),
		Type:     eventType,
		Key:      key,
		Sequence: sequence,
		Data:     event,
	}

	buff, errBuff := json.Marshal(&toPublish)
	if errBuff != nil {
		return errBuff
	}

	msg := &sarama.ProducerMessage{
		Topic: pub.topic,
		Value: sarama.ByteEncoder(buff),
	}

	_, _, errPublish := pub.prod.SendMessage(msg)
	return errPublish
}
