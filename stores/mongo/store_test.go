package mongo

import (
	"fmt"
	"os"
	"testing"

	mgo "github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/utilities/test"
	"github.com/satori/go.uuid"
)

func init() {
	bson.SetJSONTagFallback(true)
}

func provider() (eventsourcing.EventStore, func(), error) {
	collectionName := fmt.Sprintf("%s", uuid.NewV4())
	dial := os.Getenv("MONGO_TEST_HOST")
	if dial == "" {
		dial = "mongodb://localhost:27017"
	}

	result, err := NewStore(Endpoint{
		DialURL:        dial,
		DatabaseName:   "TestDatabase",
		CollectionName: collectionName,
	})

	return result, func() {
		// Connect to the MongoDB services
		session, errSession := mgo.Dial(dial)
		if errSession != nil {
			return
		}
		session.DB("TestDatabase").DropDatabase()
	}, err
}

// TestStoreCompliance
func TestStoreCompliance(t *testing.T) {
	test.CheckStandardSuite(t, "MongoDB Store", provider)
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
