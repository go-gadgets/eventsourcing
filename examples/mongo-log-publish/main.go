package main

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/distribution/inproc"
	keyvalue "github.com/go-gadgets/eventsourcing/stores/key-value"
	"github.com/go-gadgets/eventsourcing/stores/mongo"
	"github.com/go-gadgets/eventsourcing/utilities/test"
	uuid "github.com/satori/go.uuid"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
)

func init() {
	bson.SetJSONTagFallback(true)
}

func main() {
	dialURL := "mongodb://localhost:27017"
	database := "ExampleDatabase"
	collection := "TestCollection"

	// Create the inner publisher. Events get picked up from Mongo and pumped
	// into this in order to be dispatched.
	reciever := inproc.Create(test.GetTestRegistry())
	handler := &exampleHandler{}
	handler.Initialize(test.GetTestRegistry(), handler)
	reciever.AddHandler(handler)
	reciever.Start()
	defer reciever.Stop()

	// Create the oplog tailer, which picks up events and puts them into
	// the inner provider
	tracker, errTracker := mongo.CreateTracker(mongo.Endpoint{
		DialURL:        dialURL,
		DatabaseName:   database,
		CollectionName: collection + "Publishers",
	}, "example-worker-01", mongo.InitialPositionEdge)
	if errTracker != nil {
		panic(errTracker)
	}

	closer, errStartup := mongo.CreateOplogPublisher(dialURL, mongo.OplogOptions{
		TargetDatabase: database,
		CollectionName: collection,
		Registry:       test.GetTestRegistry(),
		Publisher:      reciever,
		Tracker:        tracker,
	})

	if errStartup != nil {
		panic(errStartup)
	}
	defer closer()

	// Run the producer side to simulate a client sending events
	go produceDummyEvents(dialURL, database, collection)

	// trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	for {
		select {
		case <-signals:
			fmt.Println("Aborting process due to user keystrokes.")
			return
		}
	}
}

// exampleHandler is a handler we can wire up to our inproc handler to
// demonstrate we're really listening to the oplog.
type exampleHandler struct {
	eventsourcing.EventHandlerBase
}

// HandleIncrementEvent processes an increment event consumed from the oplog
func (handler *exampleHandler) HandleIncrementEvent(key string, seq int64, event test.IncrementEvent) error {
	fmt.Printf("Key=%v, Seq=%v, IncrementBy: %v\n", key, seq, event.IncrementBy)
	return nil
}

// produceDummyEvents sends events into MongoDB every second to let the log tailer
// have something to look at.
func produceDummyEvents(dialURL string, databaseName string, collectionName string) {
	// Connect to the MongoDB services
	session, errSession := mgo.Dial(dialURL)
	if errSession != nil {
		panic(errSession)
	}
	defer session.Close()

	database := session.DB(databaseName)
	collection := database.C(collectionName)
	seq := int64(0)

	key := fmt.Sprintf("%v", uuid.NewV4())

	for {
		seq++
		event := keyvalue.KeyedEvent{
			Key:       key,
			Sequence:  seq,
			EventType: eventsourcing.EventType("IncrementEvent"),
			EventData: &test.IncrementEvent{
				IncrementBy: int(seq),
			},
		}

		fmt.Println("Producer: Inserting dummy event....")
		collection.Insert(&event)

		// Run again in a second
		time.Sleep(time.Second)
	}
}
