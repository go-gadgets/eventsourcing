package eventsourcing

import (
	"reflect"
)

// The standardCommandRegistry is the default implementation of CommandRegistry that stores
// command information for an aggregate in an internally managed structure.
type standardCommandRegistry struct {
	domain   string                       // Name of the domain
	commands map[CommandType]reflect.Type // commands to type mapping
}

// NewStandardCommandRegistry creates an instance of a plain CommandRegistry that
// stores information about command types in an internal map. The string parameter
// is the name of the domain/bounded-context in which our commands live.
func NewStandardCommandRegistry(domain string) CommandRegistry {
	return &standardCommandRegistry{
		domain:   domain,
		commands: make(map[CommandType]reflect.Type),
	}
}

// CreateCommand creates a new instance of the specified command type.
func (reg standardCommandRegistry) CreateCommand(commandType CommandType) Command {
	// Look for the type in the known types map
	entry, exists := reg.commands[commandType]

	// if no type exists, assume default polymorphic
	if !exists {
		return make(map[string]interface{})
	}

	newInstance := reflect.New(entry)

	return newInstance.Interface()
}

// Domain that this registry contains commands for.
func (reg standardCommandRegistry) Domain() string {
	return reg.domain
}

// RegisterCommand registers an command type with the registry
func (reg standardCommandRegistry) RegisterCommand(command Command) CommandType {
	commandTypeValue := reflect.TypeOf(command)
	commandTypeName := NormalizeTypeName(commandTypeValue.String())
	commandType := CommandType(commandTypeName)
	reg.commands[commandType] = commandTypeValue
	return commandType
}

// GetCommandType determines the command type label for a given command instance.
func (reg standardCommandRegistry) GetCommandType(command interface{}) (CommandType, bool) {
	commandTypeValue := reflect.TypeOf(command)
	commandTypeName := NormalizeTypeName(commandTypeValue.String())
	commandType := CommandType(commandTypeName)
	_, found := reg.commands[commandType]
	return commandType, found
}
