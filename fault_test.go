package eventsourcing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConcurrencyFault checks that a concurrency fault is correct.
func TestConcurrencyFault(t *testing.T) {
	fault := NewConcurrencyFault("dummy-key", 1234)
	assert.Equal(t, fault.Error(), "ConcurrencyFault: dummy-key at 1234", "The ConcurrencyFault message should be correct.")
	isConcurrencyFault, _ := IsConcurrencyFault(fault)
	assert.True(t, isConcurrencyFault, "Should be a ConcurrencyFault")

	isDomainFault, _ := IsDomainFault(fault)
	assert.False(t, isDomainFault, "Should not be a DomainFault")
}

// TestDomainFault checks that a domain fault is correct.
func TestDomainFault(t *testing.T) {
	fault := NewDomainFault("foo-key", "dummy-code")
	assert.Equal(t, fault.Error(), "DomainFault: dummy-code on foo-key", "The DomainFault message should be correct.")
	isConcurrencyFault, _ := IsConcurrencyFault(fault)
	assert.False(t, isConcurrencyFault, "Should not be a ConcurrencyFault")
	isDomainFault, _ := IsDomainFault(fault)
	assert.True(t, isDomainFault, "Should be a DomainFault")
}
