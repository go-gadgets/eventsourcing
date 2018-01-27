package eventsourcing

import "fmt"

// ConcurrencyFault represents an error that occured when updating an aggregate:
// specifically that we have tried to insert events at an index that is already
// defined. This means the client likely needs to re-run the command to break
// the deadlock, as someone else executed first.
type ConcurrencyFault struct {
	AggregateKey  string `json:"aggregate_key"`
	EventSequence int64  `json:"event_sequence"`
}

// Error returns the ConcurrencyFault formatted as a string to meet the Error interface.
func (curr ConcurrencyFault) Error() string {
	return fmt.Sprintf("ConcurrencyFault: %v at %v", curr.AggregateKey, curr.EventSequence)
}

// NewConcurrencyFault creates an error from the specified fault code
func NewConcurrencyFault(aggregateKey string, eventSequence int64) error {
	return ConcurrencyFault{
		AggregateKey:  aggregateKey,
		EventSequence: eventSequence,
	}
}

// IsConcurrencyFault determines if the specified error is a ConcurrencyFault
func IsConcurrencyFault(err error) (bool, *ConcurrencyFault) {
	instance, ok := err.(ConcurrencyFault)
	if ok {
		return true, &instance
	}
	return false, nil
}

// DomainFault represents an error that has arisen during a command
// that indicates the command is invalid within the domain. This can be
// any application-relevant incident (i.e. attempting to overdraw a a bank
// account)
type DomainFault struct {
	// AggregateKey that had the fault
	AggregateKey string `json:"aggregate_key"`

	// FaultCode for the domain fault
	FaultCode string `json:"fault_code"`
}

// Error returns the DomainFault formatted as a string to meet the Error interface.
func (curr DomainFault) Error() string {
	return fmt.Sprintf("DomainFault: %v on %v", curr.FaultCode, curr.AggregateKey)
}

// NewDomainFault creates an error from the specified fault code
func NewDomainFault(aggregateKey string, faultCode string) error {
	return DomainFault{
		AggregateKey: aggregateKey,
		FaultCode:    faultCode,
	}
}

// IsDomainFault determines if the specified error is a DomainFault
func IsDomainFault(err error) (bool, *DomainFault) {
	instance, ok := err.(DomainFault)
	if ok {
		return true, &instance
	}
	return false, nil
}
