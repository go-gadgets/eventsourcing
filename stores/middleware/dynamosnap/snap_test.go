package dynamosnap

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/stores/memory"
	"github.com/go-gadgets/eventsourcing/utilities/test"
)

func provider() (eventsourcing.EventStore, func(), error) {
	session, errSession := session.NewSession(&aws.Config{
		Endpoint: aws.String("http://localhost:8000"),
		Region:   aws.String("ap-southeast-2"),
	})
	if errSession != nil {
		return nil, nil, errSession
	}

	base := memory.NewStore()
	wrapped := eventsourcing.NewMiddlewareWrapper(base)
	mw, err := CreateWithSession(Parameters{
		SnapInterval: 5,
	}, session, "test-snap")
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
	test.CheckStandardSuite(t, "DynamoDB Snap Middleware", provider)
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
