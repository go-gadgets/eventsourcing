package mongosnap

import (
	"fmt"
	"os"
	"testing"

	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/stores/memory"
	"github.com/go-gadgets/eventsourcing/utilities/test"
	uuid "github.com/satori/go.uuid"
)

func provider() (eventsourcing.EventStore, func(), error) {
	collectionName := fmt.Sprintf("%s", uuid.NewV4())
	dial := os.Getenv("MONGO_TEST_HOST")
	if dial == "" {
		dial = "mongodb://localhost:27017"
	}

	base := memory.NewStore()
	wrapped := eventsourcing.NewMiddlewareWrapper(base)
	mw, err := Create(Parameters{
		SnapInterval: 5,
	}, Endpoint{
		DialURL:        dial,
		DatabaseName:   "TestDatabase",
		CollectionName: collectionName,
	})
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
	test.CheckStandardSuite(t, "MongoDB Snap Middleware", provider)
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
