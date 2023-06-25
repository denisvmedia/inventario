package errkit

import (
	"encoding/json"
	"errors"
	"fmt"
)

var (
	_ error          = (*fieldError)(nil)
	_ json.Marshaler = (*fieldError)(nil)
)

type stackTracedError struct {
	err        error      // The wrapped error
	stackTrace stackTrace // Stack trace associated with the error
}

// WithStackTrace creates a new error with a stack trace.
// It wraps the given error with the stack trace.
func WithStackTrace(err error) error {
	stack, _ := getStackTrace(1)

	return &stackTracedError{
		stackTrace: stack,
		err:        err,
	}
}

// Error implements the error interface and returns the error message.
// It includes only the wrapped error message, without the stack trace.
func (e *stackTracedError) Error() string {
	return fmt.Sprintf("%s", e.err.Error())
}

// Is implements the errors.Is interface and returns true if the target error is found.
// It checks if the target error matches the wrapped error.
func (e *stackTracedError) Is(target error) bool {
	return errors.Is(e.err, target)
}

// As implements the errors.As interface and returns true if the target error is found.
// It checks if the target error can be assigned to the wrapped error.
func (e *stackTracedError) As(target any) bool {
	return errors.As(e.err, target)
}

// Unwrap implements the errors.Wrapper interface and returns the wrapped error.
// It provides access to the original error.
func (e *stackTracedError) Unwrap() error {
	return e.err
}

// MarshalJSON implements the json.Marshaler interface and returns the serialized error.
// It serializes the error and its associated stack trace to JSON.
func (e *stackTracedError) MarshalJSON() ([]byte, error) {
	type jsonError struct {
		Error      json.RawMessage `json:"error"`                // Serialized error
		StackTrace stackTrace      `json:"stackTrace,omitempty"` // Serialized stack trace
	}

	errData, err := MarshalError(e.err) // Assuming MarshalError is a custom function to serialize the error
	if err != nil {
		return nil, err
	}

	jerr := jsonError{
		Error:      errData,
		StackTrace: e.stackTrace,
	}

	return json.Marshal(jerr) // Serialize the error and stack trace to JSON
}
