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

type fieldError struct {
	fields Fields // Additional fields associated with the error
	err    error  // The wrapped error
}

// WithFields creates a new error with additional fields.
// It wraps the given error with the provided fields.
func WithFields(err error, fields Fields) error {
	return &fieldError{
		fields: fields,
		err:    err,
	}
}

// Error implements the error interface and returns the error message.
// It includes the wrapped error message and the additional fields.
func (e *fieldError) Error() string {
	return fmt.Sprintf("%s (%+v)", e.err.Error(), e.fields)
}

// Is implements the errors.Is interface and returns true if the target error is found.
// It checks if the target error matches the wrapped error.
func (e *fieldError) Is(target error) bool {
	return errors.Is(e.err, target)
}

// As implements the errors.As interface and returns true if the target error is found.
// It checks if the target error can be assigned to the wrapped error.
func (e *fieldError) As(target any) bool {
	return errors.As(e.err, target)
}

// Unwrap implements the errors.Wrapper interface and returns the wrapped error.
// It provides access to the original error.
func (e *fieldError) Unwrap() error {
	return e.err
}

// MarshalJSON implements the json.Marshaler interface and returns the serialized error.
// It serializes the error and its additional fields to JSON.
func (e *fieldError) MarshalJSON() ([]byte, error) {
	type jsonError struct {
		Error  json.RawMessage `json:"error"`            // Serialized error
		Fields Fields          `json:"fields,omitempty"` // Serialized additional fields
	}

	errData, err := MarshalError(e.err) // Assuming MarshalError is a custom function to serialize the error
	if err != nil {
		return nil, err
	}

	jerr := jsonError{
		Error:  errData,
		Fields: e.fields,
	}

	return json.Marshal(jerr) // Serialize the error and additional fields to JSON
}
