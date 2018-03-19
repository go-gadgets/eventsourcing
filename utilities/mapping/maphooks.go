package mapping

import (
	"reflect"
	"time"
)

// MapTimeFromJSON is a decoder hook that maps time data from JSON values, avoiding the issue
// of things appearing as errors/blank when dealing with native Go time types. This is based on
// the code at https://github.com/mitchellh/mapstructure/issues/41
func MapTimeFromJSON(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
	if t == reflect.TypeOf(time.Time{}) && f == reflect.TypeOf("") {
		return time.Parse(time.RFC3339, data.(string))
	}

	return data, nil
}
