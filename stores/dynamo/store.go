package dynamo

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/stores/key-value"
)

// eventStore is a type that represents a DynamoDB backed
// EventStore implementation
type eventStore struct {
	session   *session.Session
	service   *dynamodb.DynamoDB
	tableName string
}

// NewStore creates a new DynamoDB backed event-store to use, using the default
// contextual session from the application.
func NewStore(tableName string) (eventsourcing.EventStore, error) {
	session, errSession := session.NewSession()
	if errSession != nil {
		return nil, errSession
	}

	return NewStoreWithSession(session, tableName)
}

// NewStoreWithSession creates a new DynamoDB event store, using the specified session.
func NewStoreWithSession(session *session.Session, tableName string) (eventsourcing.EventStore, error) {
	svc := dynamodb.New(session)

	engine := &eventStore{
		session:   session,
		service:   svc,
		tableName: tableName,
	}

	store := keyvalue.NewStore(keyvalue.Options{
		CheckSequence: engine.checkExists,
		FetchEvents:   engine.fetchEvents,
		PutEvents:     engine.putEvents,
		Close: func() error {
			return nil
		},
	})

	return store, nil
}

// checkExists checks that a particular sequence number exists in the store.
func (store *eventStore) checkExists(key string, seq int64) (bool, error) {
	input := &dynamodb.GetItemInput{
		ConsistentRead: aws.Bool(true),
		Key: map[string]*dynamodb.AttributeValue{
			"aggregate_key": {
				S: aws.String(key),
			},
			"seq": {
				N: aws.String(fmt.Sprintf("%d", seq)),
			},
		},
		TableName: aws.String(store.tableName),
	}

	result, errResult := store.service.GetItem(input)
	if errResult != nil {
		return false, errResult
	}

	return result.Item != nil, nil
}

// putEvents writes events to the backing store.
func (store *eventStore) putEvents(events []keyvalue.KeyedEvent) error {
	for _, v := range events {
		// Marshal the items
		av, errMarshal := dynamodbattribute.MarshalMap(v)
		if errMarshal != nil {
			return errMarshal
		}

		// Deal with Dynamo API limits around field names
		av["aggregate_key"] = av["key"]
		av["seq"] = av["sequence"]
		delete(av, "key")
		delete(av, "sequence")

		// Store the item: Need to do 1 at a time, since we don't have
		// ConditionExpression on a batch
		_, errPut := store.service.PutItem(&dynamodb.PutItemInput{
			Item:                av,
			ConditionExpression: aws.String("attribute_not_exists(aggregate_key) AND attribute_not_exists(seq)"),
			TableName:           aws.String(store.tableName),
		})

		// No error? Spin on
		if errPut == nil {
			continue
		}

		// If it's an AWS error, validate
		errAWS, ok := errPut.(awserr.Error)
		if ok {
			// AWS error?
			if errAWS.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return eventsourcing.NewConcurrencyFault(v.Key, v.Sequence)
			}
		}

		return errPut
	}

	return nil
}

// Fetch events from the store
func (store *eventStore) fetchEvents(key string, seq int64) ([]keyvalue.KeyedEvent, error) {
	loaded := make([]keyvalue.KeyedEvent, 0)
	var failure error

	errQuery := store.service.QueryPages(&dynamodb.QueryInput{
		ConsistentRead: aws.Bool(true),

		KeyConditions: map[string]*dynamodb.Condition{
			"aggregate_key": {
				ComparisonOperator: aws.String("EQ"),
				AttributeValueList: []*dynamodb.AttributeValue{
					{
						S: aws.String(key),
					},
				},
			},
			"seq": {
				ComparisonOperator: aws.String("GT"),
				AttributeValueList: []*dynamodb.AttributeValue{
					{
						N: aws.String(fmt.Sprintf("%d", seq)),
					},
				},
			},
		},
		TableName: aws.String(store.tableName),
	}, func(output *dynamodb.QueryOutput, last bool) bool {
		// Iterate through items
		for _, item := range output.Items {
			target := keyvalue.KeyedEvent{}

			// Deal with Dynamo API limits around field names
			item["key"] = item["aggregate_key"]
			item["sequence"] = item["seq"]

			errUnmarshal := dynamodbattribute.UnmarshalMap(item, &target)

			// If there was an error loading an event, stop
			if errUnmarshal != nil {
				failure = errUnmarshal
				return false
			}

			loaded = append(loaded, target)
		}

		// Continue if we have a LastEvaluatedKey
		return output.LastEvaluatedKey != nil && len(output.LastEvaluatedKey) != 0
	})

	// If there was an error, prevent people seeing the outcome
	if failure != nil {
		loaded = nil
		errQuery = failure
	}

	return loaded, errQuery
}
