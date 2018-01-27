package main

import (
	"github.com/gin-gonic/gin"
	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/stores/mongo"
)

func main() {
	gin.SetMode(gin.ReleaseMode)

	// Initialze the event store
	store, errStore := mongo.NewMongoStore(mongo.StoreParameters{
		DialURL:        "mongodb://mongodb-test:27017",
		DatabaseName:   "eventsourcingExample",
		CollectionName: "Counters",
	})
	if errStore != nil {
		panic(errStore)
	}

	// Wrap the event store in a snapshot wrapper
	store, errSnap := mongo.NewSnapStore(mongo.SnapParameters{
		DialURL:        "mongodb://mongodb-test:27017",
		DatabaseName:   "eventsourcingExample",
		CollectionName: "Counters-Snapshot",
		SnapInterval:   10,
	}, store)
	if errSnap != nil {
		panic(errSnap)
	}

	r := gin.Default()
	r.GET("/:name/increment", func(c *gin.Context) {
		name := c.Param("name")

		var count int

		errRun := eventsourcing.Retry(100, func() error {
			agg := CounterAggregate{}
			agg.Initialize(name, store, func() interface{} { return &agg })
			errRun := agg.Run(func() error {
				agg.Increment()
				return nil
			})
			if errRun != nil {
				return errRun
			}

			count = agg.Count

			return nil
		})

		if errRun != nil {
			c.JSON(500, errRun.Error())
			return
		}

		// Show the count
		c.JSON(200, gin.H{
			"count": count,
		})
	})
	r.Run() // listen and serve on 0.0.0.0:8080
}
