package errkit

import (
	"errors"
	"strings"
)

type multipleErrors interface {
	Unwrap() []error
}

type multiError struct {
	errs []error
}

// Error implements the error interface and returns the concatenated error messages.
// It joins the error messages of all errors in the slice with a newline separator.
func (e *multiError) Error() string {
	var errorMessages []string
	for _, err := range e.errs {
		errorMessages = append(errorMessages, err.Error())
	}
	return strings.Join(errorMessages, "\n")
}

// Is implements the errors.Is interface and returns true if the target error is found.
// It checks if any error in the slice matches the target error.
func (e *multiError) Is(target error) bool {
	for _, err := range e.errs {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

func (e *multiError) Unwrap() []error {
	return e.errs
}

// As implements the errors.As interface and returns true if the target error is found.
// It checks if any error in the slice can be assigned to the target error.
func (e *multiError) As(target any) bool {
	if target == nil {
		return false
	}

	for _, err := range e.errs {
		if errors.As(err, target) {
			return true
		}
	}

	return false
}

// MarshalJSON implements the json.Marshaler interface and returns the serialized errors.
// It serializes each error in the slice and returns them as a JSON array.
func (e *multiError) MarshalJSON() ([]byte, error) {
	return marshalMultiple(e)
}

// Append appends one or more errors into a single slice.
// It will return nil if err is nil.
// It will merge the errors implementing `interface { Unwrap() []error }` by calling Unwrap.
// It will merge the errors from the previous Append call in a single slice.
func Append(err error, errs ...error) error {
	if err == nil {
		return nil
	}

	if len(errs) == 0 {
		return &multiError{errs: []error{err}}
	}

	var prevErrs []error
	switch verr := err.(type) {
	case *multiError:
		prevErrs = verr.errs
	case multipleErrors:
		prevErrs = verr.Unwrap()
	default:
		prevErrs = []error{err}
	}

	newErrs := make([]error, 0, len(prevErrs)+len(errs))
	newErrs = append(newErrs, prevErrs...)
	newErrs = append(newErrs, errs...)

	return &multiError{errs: newErrs}
}

// Join merges one or more errors to the Errors slice.
func Join(errs ...error) error {
	if len(errs) == 0 {
		return nil
	}

	newErrs := make([]error, 0, len(errs))
	for _, e := range errs {
		if e != nil {
			newErrs = append(newErrs, e)
		}
	}

	switch len(newErrs) {
	case 0:
		return nil
	case 1:
		return Append(newErrs[0])
	default:
		return Append(newErrs[0], newErrs[1:]...)
	}
}
