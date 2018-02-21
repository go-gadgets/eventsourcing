package mongo

import (
	"fmt"
	"os"
	"testing"

	"github.com/globalsign/mgo/bson"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
)

func init() {
	bson.SetJSONTagFallback(true)
}

// TestTrackerWriteRead checks the oplog tracker can write then read back
func TestTrackerWriteRead(t *testing.T) {
	collectionName := fmt.Sprintf("%s", uuid.NewV4())
	dial := os.Getenv("MONGO_TEST_HOST")
	if dial == "" {
		dial = "mongodb://localhost:27017"
	}

	result, errCreate := CreateTracker(Endpoint{
		DialURL:        dial,
		DatabaseName:   "TestDatabase",
		CollectionName: collectionName,
	}, "test-tracker", InitialPositionEdge)
	assert.Nil(t, errCreate)

	initial, errInitial := result.StartPosition()
	assert.Nil(t, errInitial)
	assert.Equal(t, int64(InitialPositionEdge), initial)

	errUpdate := result.UpdatePosition(int64(1234))
	assert.Nil(t, errUpdate)

	updated, errRefetch := result.StartPosition()
	assert.Nil(t, errRefetch)
	assert.Equal(t, int64(1234), updated)
}

// BenchmarkOpLogTracker checks how many position updates we can do in a given
// time, allowing us to be confident when we tail a log.
func BenchmarkOplogTracker(b *testing.B) {
	collectionName := fmt.Sprintf("%s", uuid.NewV4())
	dial := os.Getenv("MONGO_TEST_HOST")
	if dial == "" {
		dial = "mongodb://localhost:27017"
	}

	result, errInitial := CreateTracker(Endpoint{
		DialURL:        dial,
		DatabaseName:   "TestDatabase",
		CollectionName: collectionName,
	}, "test-tracker", InitialPositionEdge)
	if errInitial != nil {
		assert.Nil(b, errInitial)
		return
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		errUpdate := result.UpdatePosition(int64(n))
		assert.Nil(b, errUpdate)
	}
}
