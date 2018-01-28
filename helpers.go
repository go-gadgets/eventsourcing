package eventsourcing

import "strings"

// Retry retries a block of code, until it hits a limit or the concurrency fault does not occur.
func Retry(limit int, body func() error) error {
	count := 1
	var lastError error
	for {
		lastError = body()

		// Succcess?
		if lastError == nil {
			return nil
		}

		// Non-concurrency error?
		concErr, _ := IsConcurrencyFault(lastError)
		if !concErr {
			return lastError
		}

		// If we've hit the limit, give up.
		if count == limit {
			return lastError
		}

		count++
	}
}

// NormalizeEventName the event name of an event so that we remove the go-supplied package name
func NormalizeEventName(name string) string {
	segments := strings.Split(name, ".")
	return segments[len(segments)-1]
}
