package redissnap

import (
	"encoding/json"
	"time"

	"github.com/globalsign/mgo/bson"
	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/stores/middleware/snapbase"
	"github.com/go-redis/redis"
)

func init() {
	bson.SetJSONTagFallback(true)
}

// Snapshot is the current snapshot for an entity, a JSON structure
// that can be persisted to the Redis instance.
type snapshot struct {
	Sequence int64       `json:"seq"`
	State    interface{} `json:"state"`
}

// Parameters describes the parameters that can be
// used to cofigure a Redis snap store.
type Parameters struct {
	Lazy            bool  // Lazy mode?
	SnapInterval    int64 `json:"snap_interval"` // SnapInterval is the number of events between snaps
	DefaultDuration time.Duration
}

// instance is our storage provider for managing snapshots in redis
type instance struct {
	client *redis.Client
	params Parameters
}

// Create a snap provider using the default connection (localhost)
func Create(params Parameters, address string) (eventsourcing.MiddlewareFactory, error) {
	client := redis.NewClient(&redis.Options{
		Addr: address,
	})

	return CreateWithClient(params, client)
}

// CreateWithClient provisions a new instance of the dynamo-snap provider using
// an existing client
func CreateWithClient(params Parameters, client *redis.Client) (eventsourcing.MiddlewareFactory, error) {
	snaps := &instance{
		client: client,
		params: params,
	}

	return func() (eventsourcing.CommitMiddleware, eventsourcing.RefreshMiddleware, eventsourcing.CloseMiddleware) {
		return snapbase.Create(snapbase.Parameters{
			Lazy:         params.Lazy,
			SnapInterval: params.SnapInterval,
			Close: func() error {
				client.Close()
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

	// Fetch from redis
	val, errGet := mw.client.Get(key).Result()
	if errGet != nil {
		if errGet == redis.Nil {
			return nil, 0, nil
		}
		return nil, 0, errGet
	}

	data := []byte(val)
	errUnmarshal := json.Unmarshal(data, &loaded)
	if errUnmarshal != nil {
		return nil, 0, errUnmarshal
	}

	return loaded.State, loaded.Sequence, nil
}

// purge a key from the cache
func (mw *instance) purge(key string) error {
	// Delete from redis
	_, errDelete := mw.client.Del(key).Result()
	return errDelete
}

// put an item into the cache
func (mw *instance) put(key string, seq int64, data interface{}) error {
	snap := snapshot{
		Sequence: seq,
		State:    data,
	}

	// Marshal the items
	buf, errMarshal := json.Marshal(&snap)
	if errMarshal != nil {
		return errMarshal
	}

	text := string(buf)
	errPut := mw.client.Set(key, text, mw.params.DefaultDuration).Err()

	return errPut
}
