package test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/stores/memory"
	uuid "github.com/satori/go.uuid"
)

// AggregateTester is a harness that allows for aggregates to be tested
type AggregateTester struct {
	commands map[string]CommandFactory
	factory  func(key string, store eventsourcing.EventStore) eventsourcing.AggregateBase
}

// RegisterCommand registers a command with the aggregate tester, allowing it to be summoned
// during testing. Generally there is no other reason to summon commands dynamically.
func (tester *AggregateTester) RegisterCommand(name string, factory CommandFactory) {
	if tester.commands == nil {
		tester.commands = make(map[string]CommandFactory)
	}
	tester.commands[name] = factory
}

// SetAggregateFactory sets the aggregate factory instance for this type
func (tester *AggregateTester) SetAggregateFactory(factory func(key string, store eventsourcing.EventStore) eventsourcing.AggregateBase) {
	tester.factory = factory
}

// Run executes an aggregate test
func (tester *AggregateTester) Run(t *testing.T, test AggregateTest) error {
	aggregateKey := uuid.NewV4().String()
	store := memory.NewStore()

	// Iterate steps
	for _, step := range test.Commands {
		// Create the command
		cmd, errParseCommand := tester.commands[step.Type](step.Data)
		t.Logf("   --> %v: %v, ", step.Type, cmd)

		if errParseCommand != nil {
			return errParseCommand
		}

		// Get the aggregate
		agg := tester.factory(aggregateKey, store)
		errLoad := agg.Refresh()
		if errLoad != nil {
			return errLoad
		}

		errCmd := agg.Handle(cmd)
		if errCmd != nil {
			if step.Error != "" && strings.Contains(errCmd.Error(), step.Error) {
				t.Logf("       (Found error, as expected: %v)", step.Error)
			} else {
				return errCmd
			}
		}

		errCommit := agg.Commit()
		if errCommit != nil {
			return errCommit
		}
	}

	return nil
}

// CommandFactory is a method that creates a command from the JSON data
type CommandFactory func(data map[string]interface{}) (eventsourcing.Command, error)

// AggregateTests is a set of of tests for an aggregate
type AggregateTests map[string]AggregateTest

// AggregateTest is a single test for an aggregate, which
// applies a series of commands to a model and validates outcomes
type AggregateTest struct {
	Commands []AggregateTestCommand `json:"commands"` // Commands to test
}

// AggregateTestCommand is a single command to test against a model
type AggregateTestCommand struct {
	Type  string                 `json:"type"`  // Type of command to create
	Error string                 `json:"error"` // Error/fault to expect, if any
	Data  map[string]interface{} `json:"data"`  // Data for the event
}

// LoadTestsFromFile loads a set of aggregate tests
func LoadTestsFromFile(t *testing.T, fileName string) (AggregateTests, error) {
	t.Logf("Starting to run tests from file %v", fileName)
	file, errFile := os.Open(fileName)
	if errFile != nil {
		t.Error(errFile)
		return nil, errFile
	}
	defer file.Close()

	data, errRead := ioutil.ReadAll(file)
	if errRead != nil {
		t.Error(errRead)
		return nil, errRead
	}

	tests := AggregateTests{}
	errUnmarshal := json.Unmarshal(data, &tests)
	if errUnmarshal != nil {
		t.Error(errUnmarshal)
		return nil, errUnmarshal
	}

	return tests, nil
}
