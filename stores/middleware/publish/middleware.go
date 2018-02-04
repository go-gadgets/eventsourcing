package publish

import (
	"github.com/go-gadgets/eventsourcing"
)

// Create a new publishing middleware
func Create(publisher eventsourcing.EventPublisher) (eventsourcing.CommitMiddleware, eventsourcing.RefreshMiddleware, func() error) {
	return func(writer eventsourcing.StoreWriterAdapter, next eventsourcing.NextHandler) error {
			// Get the events we're about to publish
			key := writer.GetKey()
			seq, events := writer.GetUncommittedEvents()

			// Run the upstream, and abort if we don't succeed.
			errNext := next()
			if errNext != nil {
				return errNext
			}

			// Call the publisher for each event
			for index, event := range events {
				seq := seq + int64(1+index)
				errPublish := publisher.Publish(key, seq, event)
				if errPublish != nil {
					return errPublish
				}
			}

			return nil
		}, func(reader eventsourcing.StoreLoaderAdapter, next eventsourcing.NextHandler) error {
			// Call next directly
			return next()
		}, func() error {
			return nil
		}
}
