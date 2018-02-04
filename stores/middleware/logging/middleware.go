package logging

import (
	"github.com/sirupsen/logrus"

	"github.com/go-gadgets/eventsourcing"
)

// Create a new logrus middleware
func Create() (eventsourcing.CommitMiddleware, eventsourcing.RefreshMiddleware, func() error) {
	call := 0
	return func(writer eventsourcing.StoreWriterAdapter, next eventsourcing.NextHandler) error {
			count, _ := writer.GetUncommittedEvents()
			logger := logrus.WithFields(logrus.Fields{
				"key":    writer.GetKey(),
				"seq":    writer.SequenceNumber(),
				"call":   call,
				"events": count,
			})
			call++

			logger.Debug("commit_start")
			errNext := next()
			if errNext != nil {
				logger.WithFields(logrus.Fields{
					"error": errNext,
				}).Error("commit_error")
				return errNext
			}

			logger.Debug("commit_complete")
			return nil
		}, func(reader eventsourcing.StoreLoaderAdapter, next eventsourcing.NextHandler) error {
			logger := logrus.WithFields(logrus.Fields{
				"key":  reader.GetKey(),
				"seq":  reader.SequenceNumber(),
				"call": call,
			})
			call++

			logger.Debug("refresh_start")

			errNext := next()
			if errNext != nil {
				logger.WithFields(logrus.Fields{
					"error": errNext,
				}).Error("refresh_error")
				return errNext
			}

			logger.Debug("refresh_complete")
			return nil
		}, func() error {
			logrus.Debug("middleware_shutdown")
			return nil
		}
}
