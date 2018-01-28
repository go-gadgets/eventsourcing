package bson

// useJSONTagFallback lets the BSON decoder fall back to using the json tag on a structure if there
// is a JSON tag present.
var useJSONTagFallback = false

// SetJSONFallback enables or disables the JSON fallback for a structure. Due to the nature of MGO's decoder,
// this is a global setting that needs to be enabled application by application.
func SetJSONFallback(state bool) {
	useJSONTagFallback = state
}
