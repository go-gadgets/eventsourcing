package mongo

import (
	"fmt"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/go-gadgets/eventsourcing"
	keyvalue "github.com/go-gadgets/eventsourcing/stores/key-value"
	"github.com/mitchellh/mapstructure"
	"github.com/rwynn/gtm"
	"github.com/sirupsen/logrus"
)

// oplogPublisher is a MongDB oplog tailer that chases the MongoDB oplog for a set
// of collections and pushes them into a target event publisher.
type oplogPublisher struct {
	ctx        *gtm.OpCtx                   // Oplog context
	collection string                       // Collection to watch
	database   string                       // Database to watch
	inner      eventsourcing.EventPublisher // Event publisher
	registry   eventsourcing.EventRegistry  // Event registry
	terminate  chan bool                    // Termination channel
	tracker    ProgressTracker              // Position tracker
}

// OplogOptions contains the options for tailing an oplog.
type OplogOptions struct {
	TargetDatabase string                       // TargetDatabase is the database to read
	CollectionName string                       // Collection name
	Publisher      eventsourcing.EventPublisher // Event publisher
	Registry       eventsourcing.EventRegistry  // Event registry
	Tracker        ProgressTracker              // Progress tracker
}

// CreateOplogPublisher creates a new publisher that consumes events from a MongoDB
// oplog and propegates them to a target.
func CreateOplogPublisher(dialURL string, options OplogOptions) (func() error, error) {
	// Check we can comnnect to the dial URL
	session, err := mgo.Dial(dialURL)
	if err != nil {
		return nil, err
	}
	return CreateOpLogPublisherFromSession(session, options)
}

// CreateOpLogPublisherFromSession creates a new publisher that consumes events from a MongoDB
// oplog and propegates them to a target. This version allows BYO sessions.
func CreateOpLogPublisherFromSession(session *mgo.Session, options OplogOptions) (func() error, error) {
	// Validate BSON tag fallback global state
	if !bson.JSONTagFallbackState() {
		return nil, fmt.Errorf("You must configure bson.SetJSONTagFallback(true) to use this driver")
	}

	session.SetMode(mgo.Monotonic, true)
	initial, errInitial := options.Tracker.StartPosition()
	if errInitial != nil {
		return nil, errInitial
	}

	// Start listening, filtering to the specific collecton
	ctx := gtm.Start(session, &gtm.Options{
		After: func(session *mgo.Session, options *gtm.Options) bson.MongoTimestamp {
			if initial == InitialPositionTrimHorizon {
				return bson.MongoTimestamp(0)
			} else if initial == InitialPositionEdge {
				return gtm.LastOpTimestamp(session, nil)
			}
			return bson.MongoTimestamp(initial)
		},
		WorkerCount: 8,         // defaults to 1. number of go routines batch fetching concurrently
		Ordering:    gtm.Oplog, // defaults to gtm.Oplog. ordering guarantee of events on the output channel
	})

	// Shutdown signaller
	signals := make(chan bool, 1)
	terminator := func() error {
		signals <- true
		return nil
	}

	pub := &oplogPublisher{
		ctx:        ctx,
		collection: options.CollectionName,
		database:   options.TargetDatabase,
		inner:      options.Publisher,
		registry:   options.Registry,
		terminate:  signals,
		tracker:    options.Tracker,
	}

	go pub.runOpLogPublisher()

	return terminator, nil
}

func (pub *oplogPublisher) runOpLogPublisher() {
	logrus.Info("Starting to tail MongoDB oplog...")

	for {
		// loop forever receiving events
		select {
		case <-pub.terminate:
			logrus.Info("Recieved shutdown signal, exiting.")
			return

		case err := <-pub.ctx.ErrC:
			// handle errors
			logrus.Error(err)
			time.Sleep(time.Second)

		case op := <-pub.ctx.OpC:
			switch {
			// If we're not interested, skip it
			case op.Data == nil:
				fallthrough
			case !op.IsInsert():
				fallthrough
			case op.GetDatabase() != pub.database:
				fallthrough
			case op.GetCollection() != pub.collection:
				continue
			default:
				break
			}

			event, errEvent := decodeOpLogEntry(op.Data, pub.registry)
			if errEvent != nil {
				logrus.WithFields(logrus.Fields{
					"error": errEvent,
				}).Warn("Skipping event (Unable to decode)")
				break
			}

			errPublish := pub.inner.Publish(event.Key, event.Sequence, event.EventData)
			if errPublish != nil {
				logrus.Error(errPublish)
				continue
			}

			errUpdate := pub.tracker.UpdatePosition(int64(op.Timestamp))
			if errUpdate != nil {
				logrus.Error(errUpdate)
				continue
			}
		}
	}
}

// decodeOpLogEntry decodes an event. This involves taking the BSON decoded structure we've
// got from the OpLog, then performing a parse into KeyedEvent. From this we can sniff the
// event type and then perform a final pass to revive the real type under the hood.
func decodeOpLogEntry(data map[string]interface{}, registry eventsourcing.EventRegistry) (keyvalue.KeyedEvent, error) {
	event := keyvalue.KeyedEvent{}

	// Decode the wrapper
	wrapperConfig := &mapstructure.DecoderConfig{
		TagName:          "json",
		Result:           &event,
		WeaklyTypedInput: true,
	}
	wrapperDecoder, errWrapper := mapstructure.NewDecoder(wrapperConfig)
	if errWrapper != nil {
		return event, errWrapper
	}
	errDecodeWrapper := wrapperDecoder.Decode(data)
	if errDecodeWrapper != nil {
		return event, errDecodeWrapper
	}

	// Create the target type and decode into it
	summoned := registry.CreateEvent(event.EventType)
	config := &mapstructure.DecoderConfig{
		TagName:          "json",
		Result:           summoned,
		WeaklyTypedInput: true,
	}
	decoder, errDecoderInner := mapstructure.NewDecoder(config)
	if errDecoderInner != nil {
		return event, errDecoderInner
	}
	errDecode := decoder.Decode(event.EventData)
	if errDecode != nil {
		return event, errDecode
	}

	event.EventData = summoned
	return event, nil
}

// InitialPosition of the tracker
type InitialPosition int64

const (

	// InitialPositionTrimHorizon is a constant that indicates a tracker starts at the beginning of time
	InitialPositionTrimHorizon = int64(-2)

	// InitialPositionEdge indicates a tracker starts at the most recent event and works forward
	InitialPositionEdge = int64(-1)
)

// ProgressTracker is an interface that describes a mechanism that stores
// the current progress of an OpLog follower and logs progress.
type ProgressTracker interface {
	// StartPosition fetches the initial offset to tail from the log
	StartPosition() (int64, error)

	// UpdatePosition sets the target position for an Oplog tailer.
	UpdatePosition(int64) error
}

// CreateTracker creates a new MongoDB backed oplog tracker
func CreateTracker(endpoint Endpoint, key string, initialPosition int64) (ProgressTracker, error) {
	// Connect to the MongoDB services
	session, errSession := mgo.Dial(endpoint.DialURL)
	if errSession != nil {
		return nil, errSession
	}

	database := session.DB(endpoint.DatabaseName)
	collection := database.C(endpoint.CollectionName)

	return CreateTrackerWithConnection(session, collection, key, initialPosition)
}

// CreateTrackerWithConnection creates a new MGO-backed tracker with a specific connection
// and collection. Clients assume shutdown responsibility.
func CreateTrackerWithConnection(session *mgo.Session, collection *mgo.Collection, key string, initialPosition int64) (ProgressTracker, error) {
	// Ensure the index exists
	errIndex := collection.EnsureIndex(mgo.Index{
		Key:        []string{"key"},
		Unique:     true,
		DropDups:   false,
		Background: false,
	})
	if errIndex != nil {
		session.Close()
		return nil, errIndex
	}

	instance := &tracker{
		initial:    initialPosition,
		session:    session,
		collection: collection,
		key:        key,
	}

	return instance, nil
}

// tracker is a structure that contains the state for a progress tracker that
// stores information into MongoDB.
type tracker struct {
	initial    int64
	session    *mgo.Session
	collection *mgo.Collection
	key        string
}

// trackerRecord is a structure that represents the tracker position data in Mongo.
type trackerRecord struct {
	Key      string `json:"key"`      // Key (worker ID)
	Position int64  `json:"position"` // Last stored position
}

// StartPosition gets the starting position for a worker
func (tracker *tracker) StartPosition() (int64, error) {
	var result []trackerRecord
	errSequence := tracker.collection.Find(bson.M{
		"key": tracker.key,
	}).All(&result)
	if errSequence != nil {
		return 0, errSequence
	}

	if len(result) == 0 {
		return tracker.initial, nil
	}
	return result[0].Position, nil
}

// UpdatePosition stores the current position
func (tracker *tracker) UpdatePosition(position int64) error {
	_, errUpsert := tracker.collection.Upsert(bson.M{
		"key": tracker.key,
	}, bson.M{
		"key":      tracker.key,
		"position": position,
	})
	return errUpsert
}
