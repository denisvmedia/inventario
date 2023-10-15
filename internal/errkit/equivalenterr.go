package errkit

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
)

var (
	_ error          = (*equivalentError)(nil)
	_ json.Marshaler = (*equivalentError)(nil)
)

type equivalentError struct {
	err         error       // The main error
	equivalents *multiError // Equivalent errors
}

// WithEquivalents creates a new error with equivalent errors.
// It wraps the given error and appends the provided equivalent errors.
func WithEquivalents(err error, errs ...error) error {
	return &equivalentError{
		err:         err,
		equivalents: &multiError{errs: errs},
	}
}

// Error implements the error interface and returns the concatenated error messages.
// It includes the main error message and the error messages of all equivalent errors.
// The messages are joined with newline separators.
func (e *equivalentError) Error() string {
	return strings.Join(append([]string{e.err.Error()}, e.equivalents.Error()), "\n")
}

// Is implements the errors.Is interface and returns true if the target error is found.
// It checks if the target error matches the main error or any of the equivalent errors.
func (e *equivalentError) Is(target error) bool {
	if errors.Is(e.err, target) {
		return true
	}

	for _, err := range e.equivalents.errs {
		if errors.Is(err, target) {
			return true
		}
	}

	return false
}

// As implements the errors.As interface and returns true if the target error is found.
// It checks if the target error can be assigned to the main error or any of the equivalent errors.
func (e *equivalentError) As(target any) bool {
	if target == nil {
		return false
	}

	if reflect.TypeOf(target) == reflect.TypeOf(e) {
		reflect.ValueOf(target).Elem().Set(reflect.ValueOf(e))
		return true
	}

	if errors.As(e.err, target) {
		return true
	}

	for _, err := range e.equivalents.errs {
		if errors.As(err, target) {
			return true
		}
	}

	return false
}

// Unwrap implements the errors.Wrapper interface and returns the wrapped error.
// It provides access to the main error.
func (e *equivalentError) Unwrap() error {
	return e.err
}

// MarshalJSON implements the json.Marshaler interface and returns the serialized error.
// It serializes the main error and the equivalent errors to JSON.
func (e *equivalentError) MarshalJSON() ([]byte, error) {
	type jsonError struct {
		Error       json.RawMessage `json:"error"`       // Serialized main error
		Equivalents json.RawMessage `json:"equivalents"` // Serialized equivalent errors
	}

	var (
		result jsonError
		err    error
	)

	result.Error, err = json.Marshal(e.err) // Serialize the main error
	if err != nil {
		return nil, err
	}

	result.Equivalents, err = json.Marshal(e.equivalents) // Serialize the equivalent errors
	if err != nil {
		return nil, err
	}

	return json.Marshal(result) // Serialize the result struct to JSON
}
