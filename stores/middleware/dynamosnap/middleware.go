package dynamosnap

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/globalsign/mgo/bson"
	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/stores/middleware/snapbase"
)

func init() {
	bson.SetJSONTagFallback(true)
}

// Snapshot is the current snapshot for an entity, a JSON structure
// that can be persisted to the Mongo instance.
type snapshot struct {
	Key      string      `json:"aggregate_key"`
	Sequence int64       `json:"seq"`
	State    interface{} `json:"state"`
}

// Parameters describes the parameters that can be
// used to cofigure a MongoDB snap store.
type Parameters struct {
	Lazy         bool  // Lazy mode?
	SnapInterval int64 `json:"snap_interval"` // SnapInterval is the number of events between snaps
}

// instance is our storage provider for managing snapshots in memory
type instance struct {
	service   *dynamodb.DynamoDB
	params    Parameters
	tableName string
}

// Create a snap provider using the default AWS session context
func Create(params Parameters, tableName string) (eventsourcing.MiddlewareFactory, error) {
	// Create an AWS Session
	session, errSession := session.NewSession()
	if errSession != nil {
		return nil, errSession
	}

	return CreateWithSession(params, session, tableName)
}

// CreateWithSession provisions a new instance of the dynamo-snap provider using
// an existing session
func CreateWithSession(params Parameters, session *session.Session, tableName string) (eventsourcing.MiddlewareFactory, error) {
	service := dynamodb.New(session)

	snaps := &instance{
		service:   service,
		tableName: tableName,
		params:    params,
	}

	return func() (eventsourcing.CommitMiddleware, eventsourcing.RefreshMiddleware, eventsourcing.CloseMiddleware) {
		return snapbase.Create(snapbase.Parameters{
			Lazy:         params.Lazy,
			SnapInterval: params.SnapInterval,
			Close: func() error {
				return nil
			},
			Get:   snaps.get,
			Purge: snaps.purge,
			Put:   snaps.put,
		})
	}, nil
}

// get a key from the cache
func (mw *instance) get(key string) (interface{}, int64, error) {
	var loaded snapshot
	// Build query
	input := &dynamodb.GetItemInput{
		ConsistentRead: aws.Bool(true),
		Key: map[string]*dynamodb.AttributeValue{
			"aggregate_key": {
				S: aws.String(key),
			},
		},
		TableName: aws.String(mw.tableName),
	}

	// Fetch
	result, errResult := mw.service.GetItem(input)
	if errResult != nil {
		return nil, 0, errResult
	}

	if result.Item == nil {
		return loaded.State, loaded.Sequence, nil
	}

	errSnap := dynamodbattribute.UnmarshalMap(result.Item, &loaded)
	if errSnap != nil {
		return nil, 0, errSnap
	}

	return loaded.State, loaded.Sequence, nil
}

// purge a key from the cache
func (mw *instance) purge(key string) error {
	// Build input
	input := &dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"aggregate_key": {
				S: aws.String(key),
			},
		},
		TableName: aws.String(mw.tableName),
	}

	_, errPurge := mw.service.DeleteItem(input)
	return errPurge
}

// put an item into the cache
func (mw *instance) put(key string, seq int64, data interface{}) error {
	snap := snapshot{
		Key:      key,
		Sequence: seq,
		State:    data,
	}

	// Marshal the items
	av, errMarshal := dynamodbattribute.MarshalMap(snap)
	if errMarshal != nil {
		return errMarshal
	}

	_, errPut := mw.service.PutItem(&dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(mw.tableName),
	})

	return errPut
}
