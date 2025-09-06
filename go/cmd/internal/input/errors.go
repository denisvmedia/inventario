package input

import "fmt"

// ValidationError represents a validation failure
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation failed for %s: %s", e.Field, e.Message)
}

// AnswerError represents an invalid user input
type AnswerError struct {
	Message string
}

func (e AnswerError) Error() string {
	return e.Message
}

// NewAnswerError creates a new AnswerError
func NewAnswerError(message string) error {
	return AnswerError{Message: message}
}

// RequiredFieldError represents a missing required field
type RequiredFieldError struct {
	Field string
}

func (e RequiredFieldError) Error() string {
	return fmt.Sprintf("field %s is required", e.Field)
}

// NewRequiredFieldError creates a new RequiredFieldError
func NewRequiredFieldError(field string) error {
	return RequiredFieldError{Field: field}
}
