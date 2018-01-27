package eventsourcing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRetryOperation checks that the retry will attempt an operation multiple times.
func TestRetryOperation(t *testing.T) {
	count := 0

	errOutcome := Retry(10, func() error {
		count++
		if count < 5 {
			return NewConcurrencyFault("dummy-key", int64(count))
		}

		return nil
	})

	assert.Nil(t, errOutcome, "The retry should not return an error.")
	assert.Equal(t, 5, count, "The count should be 5 at the end of the test.")
}

// TestRetryBailout checks that we won't keep trying forever.
func TestRetryBailout(t *testing.T) {
	count := 0

	errOutcome := Retry(10, func() error {
		count++
		return NewConcurrencyFault("dummy-key", int64(count))
	})

	assert.NotNil(t, errOutcome, "The retry should return an error.")
	assert.Equal(t, 10, count, "The count should be 10 at the end of the test.")
}

// TestNonRetryableBailout checks that we won't keep trying if it's not a concurrenc fault
func TestNonRetryableBailout(t *testing.T) {
	count := 0

	errOutcome := Retry(10, func() error {
		count++
		return NewDomainFault("dummy-key", "bad-idea")
	})

	assert.NotNil(t, errOutcome, "The retry should return an error.")
	assert.Equal(t, 1, count, "The count should be 1 at the end of the test.")
}
