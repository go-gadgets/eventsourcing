package main

import (
	"testing"

	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/utilities/test"
)

// TestAggregateIncrement checks that the model handles increment commands up to
// the target.
func TestAggregateIncrement(t *testing.T) {
	agg := &CounterAggregate{}
	store := test.NewNullStore()
	agg.Initialize("dummy-key", store, func() interface{} { return agg })

	for x := 0; x < 30; x++ {
		errIncrement := agg.Handle(IncrementCommand{})
		if errIncrement != nil {
			t.Error(errIncrement)
			return
		}
	}

	errLimit := agg.Handle(IncrementCommand{})
	if errLimit == nil {
		t.Error("Should have got an error, got none")
		return
	}

	check, err := eventsourcing.IsDomainFault(errLimit)
	if !check {
		t.Errorf("Got incorrect error type: %v", err)
	} else if err.FaultCode != "limit_reached" {
		t.Errorf("Got incorrect fault code: %v", err.FaultCode)
	}
}
