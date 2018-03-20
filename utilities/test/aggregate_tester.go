package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-gadgets/eventsourcing"
	"github.com/go-gadgets/eventsourcing/stores/memory"
	"github.com/go-gadgets/eventsourcing/utilities/mapping"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/mitchellh/mapstructure"
	uuid "github.com/satori/go.uuid"
)

// AggregateTester is a harness that allows for aggregates to be tested
type AggregateTester struct {
	commands eventsourcing.CommandRegistry
	factory  func(key string, store eventsourcing.EventStore) eventsourcing.AggregateBase
}

// RegisterCommand registers a command with the aggregate tester, allowing it to be summoned
// during testing. Generally there is no other reason to summon commands dynamically.
func (tester *AggregateTester) RegisterCommand(command eventsourcing.Command) {
	if tester.commands == nil {
		tester.commands = eventsourcing.NewStandardCommandRegistry("test-domain")
	}
	tester.commands.RegisterCommand(command)
}

// SetAggregateFactory sets the aggregate factory instance for this type
func (tester *AggregateTester) SetAggregateFactory(factory func(key string, store eventsourcing.EventStore) eventsourcing.AggregateBase) {
	tester.factory = factory
}

// RunRecursive runs model tests recursively over a folder, loading in all
// .json files in the folder.
func (tester *AggregateTester) RunRecursive(t *testing.T, path string) error {
	// Find the test files
	testFiles := []string{}
	errWalk := filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(f.Name()), ".json") {
			return nil
		}

		testFiles = append(testFiles, path)
		return nil
	})
	if errWalk != nil {
		t.Error(errWalk)
		return errWalk
	}

	for _, file := range testFiles {
		tests, errTests := LoadTestsFromFile(t, file)
		if errTests != nil {
			t.Error(errTests)
			return errTests
		}

		for k, v := range tests {
			store := memory.NewStore()

			t.Logf(" ==> %v\n", k)

			errTest := tester.Run(t, v, tests, store)
			if errTest != nil {
				t.Errorf("Failed test %v: %v", k, errTest)
				return errTest
			}
		}
	}

	return nil
}

// Run executes an aggregate test
func (tester *AggregateTester) Run(t *testing.T, test AggregateTest, tests AggregateTests, store eventsourcing.EventStore) error {
	aggregateKey := uuid.NewV4().String()
	return tester.runInternal(t, aggregateKey, test, tests, store)
}

// runInternal runs an aggregate test
func (tester *AggregateTester) runInternal(t *testing.T, aggregateKey string, test AggregateTest, tests AggregateTests, store eventsourcing.EventStore) error {
	// If we are inheriting from another test
	if test.Inherit != "" {
		errParent := tester.runInternal(t, aggregateKey, tests[test.Inherit], tests, store)
		if errParent != nil {
			return errParent
		}
	}

	// Iterate steps
	for _, step := range test.Commands {
		// Create the command
		cmd := tester.commands.CreateCommand(eventsourcing.CommandType(step.Type))

		config := &mapstructure.DecoderConfig{
			DecodeHook:       mapping.MapTimeFromJSON,
			TagName:          "json",
			Result:           &cmd,
			WeaklyTypedInput: true,
		}
		decoder, errDecoder := mapstructure.NewDecoder(config)
		if errDecoder != nil {
			return errDecoder
		}

		errDecode := decoder.Decode(step.Data)
		if errDecode != nil {
			return errDecode
		}
		t.Logf("   --> %v: %v, ", step.Type, cmd)

		// Get the aggregate
		agg := tester.factory(aggregateKey, store)
		errLoad := agg.Refresh()
		if errLoad != nil {
			return errLoad
		}

		cmd = reflect.ValueOf(cmd).Elem().Interface()
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

	// Validate post-state
	if test.Expect != nil {
		// Get the aggregate
		agg := tester.factory(aggregateKey, store)
		errLoad := agg.Refresh()
		if errLoad != nil {
			return errLoad
		}

		// Convert JSON to target
		target := tester.factory(aggregateKey, store)
		state := target.State()
		config := &mapstructure.DecoderConfig{
			DecodeHook:       mapping.MapTimeFromJSON,
			TagName:          "json",
			Result:           &state,
			WeaklyTypedInput: true,
		}
		decoder, errDecoder := mapstructure.NewDecoder(config)
		if errDecoder != nil {
			return errDecoder
		}
		errDecode := decoder.Decode(test.Expect)
		if errDecode != nil {
			return errDecode
		}

		diff := cmp.Diff(agg.State(), state, cmpopts.IgnoreUnexported(eventsourcing.AggregateBase{}))
		if diff != "" {
			return fmt.Errorf("State validation for test failed: state did not match expected:\n%v", diff)
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
	Inherit  string                 `json:"inherit"`  // Previous test to run before this one
	Expect   map[string]interface{} `json:"expect"`   // Post-state of aggregate
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
