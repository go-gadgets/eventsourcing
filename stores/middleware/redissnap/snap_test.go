package redissnap

import (
	"testing"
	"time"

	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/stores/memory"
	"github.com/go-gadgets/eventsourcing/utilities/test"
)

func provider() (eventsourcing.EventStore, func(), error) {
	base := memory.NewStore()
	wrapped := eventsourcing.NewMiddlewareWrapper(base)
	mw, err := Create(Parameters{
		SnapInterval:    5,
		DefaultDuration: time.Hour * 24,
	}, "localhost:6379")
	if err != nil {
		return nil, nil, err
	}
	wrapped.Use(mw())

	return wrapped, func() {
		wrapped.Close()
	}, nil
}

// TestStoreCompliance
func TestStoreCompliance(t *testing.T) {
	test.CheckStandardSuite(t, "Redis Snap Middleware", provider)
}

// BenchmarkIndividualCommmits tests how fast we can apply events to an aggregate
func BenchmarkIndividualCommmits(b *testing.B) {
	test.MeasureIndividualCommits(b, provider)
}

// BenchmarkBulkInsertAndLoad tests how fast we can write
// and then load/refresh 1000 events from an aggregate
func BenchmarkBulkInsertAndLoad(b *testing.B) {
	test.MeasureBulkInsertAndReload(b, provider)
}
