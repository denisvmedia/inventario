package errkit

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
)

type Errors []error

// Error implements the error interface and returns the concatenated error messages.
// It joins the error messages of all errors in the slice with a newline separator.
func (e Errors) Error() string {
	var errorMessages []string
	for _, err := range e {
		errorMessages = append(errorMessages, err.Error())
	}
	return strings.Join(errorMessages, "\n")
}

// Is implements the errors.Is interface and returns true if the target error is found.
// It checks if any error in the slice matches the target error.
func (e Errors) Is(target error) bool {
	for _, err := range e {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// As implements the errors.As interface and returns true if the target error is found.
// It checks if any error in the slice can be assigned to the target error.
func (e Errors) As(target any) bool {
	if target == nil {
		return false
	}

	// Check if the target matches the Errors slice itself
	if reflect.TypeOf(target) == reflect.TypeOf(e) {
		reflect.ValueOf(target).Elem().Set(reflect.ValueOf(e))
		return true
	}

	for _, err := range e {
		if errors.As(err, target) {
			return true
		}
	}
	return false
}

// MarshalJSON implements the json.Marshaler interface and returns the serialized errors.
// It serializes each error in the slice and returns them as a JSON array.
func (e Errors) MarshalJSON() ([]byte, error) {
	errs := make([]json.RawMessage, 0, len(e))

	for _, v := range e {
		ev, err := MarshalError(v)
		if err != nil {
			return nil, err
		}
		errs = append(errs, ev)
	}

	return json.Marshal(errs)
}

// Append appends one or more errors to the Errors slice.
// It checks if the provided error is already an Errors slice and appends the new errors.
// Otherwise, it creates a new Errors slice and appends the provided error and new errors.
func Append(err error, errs ...error) error {
	if err == nil {
		return nil
	}

	if e := (Errors)(nil); errors.As(err, &e) {
		return append(e, errs...)
	}

	return append(Errors{err}, errs...)
}
