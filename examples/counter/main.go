package main

import (
	"github.com/gin-gonic/gin"
	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/stores/middleware/logging"
	"github.com/go-gadgets/eventsourcing/stores/middleware/memorysnap"
	"github.com/go-gadgets/eventsourcing/stores/middleware/mongosnap"
	"github.com/go-gadgets/eventsourcing/stores/mongo"
	"github.com/sirupsen/logrus"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	logrus.SetLevel(logrus.DebugLevel)

	// Initialze the event store
	mongoStore, errStore := mongo.NewStore(mongo.Endpoint{
		DialURL:        "mongodb://mongodb-test:27017",
		DatabaseName:   "eventsourcingExample",
		CollectionName: "Counters",
	})
	if errStore != nil {
		panic(errStore)
	}

	store := eventsourcing.NewMiddlewareWrapper(mongoStore)

	// Snapshotting to MongoDB
	mongoSnap, errSnap := mongosnap.Create(mongosnap.Parameters{
		SnapInterval: 10,
	}, mongosnap.Endpoint{
		DialURL:        "mongodb://mongodb-test:27017",
		DatabaseName:   "eventsourcingExample",
		CollectionName: "Counters-Snapshot",
	})
	if errSnap != nil {
		panic(errSnap)
	}
	store.Use(mongoSnap())

	// Create a lazy in-memory snapshot
	store.Use(memorysnap.Create(memorysnap.Parameters{
		Lazy:         true,
		SnapInterval: 1,
	}))

	// Logging
	store.Use(logging.Create())

	r := gin.Default()
	r.GET("/:name/increment", func(c *gin.Context) {
		name := c.Param("name")

		var count int

		errRun := eventsourcing.Retry(100, func() error {
			agg := CounterAggregate{}
			agg.Initialize(name, store, func() interface{} { return &agg })

			errRun := agg.Run(func() error {
				count = agg.Count
				return nil
			})

			return errRun
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

	r.POST("/:name/increment", func(c *gin.Context) {
		name := c.Param("name")

		errCommand := eventsourcing.Retry(10, func() error {
			agg := CounterAggregate{}
			agg.Initialize(name, store, func() interface{} { return &agg })

			err := agg.Handle(IncrementCommand{})
			if err != nil {
				return err
			}

			// Show the count
			c.JSON(200, gin.H{
				"count": agg.Count,
			})
			return nil
		})

		if errCommand != nil {
			c.JSON(500, errCommand.Error())
			return
		}
	})

	r.Run() // listen and serve on 0.0.0.0:8080
}
