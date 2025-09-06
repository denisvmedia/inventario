package input

import (
	"context"
	"errors"
	"fmt"
)

// StringField represents a string input field
type StringField struct {
	label        string
	reader       *Reader
	required     bool
	defaultValue *string
	minLength    *int
	maxLength    *int
	validator    func(string) error
	empty        bool
	value        string
}

// NewStringField creates a new string input field
func NewStringField(label string, reader *Reader) *StringField {
	return &StringField{
		label:  label,
		reader: reader,
	}
}

// SetReader sets the reader for the field (used for testing)
func (f *StringField) SetReader(reader *Reader) {
	f.reader = reader
}

// Required creates a new StringField marked as required
func (f *StringField) Required() *StringField {
	newField := *f
	newField.required = true
	return &newField
}

// Optional creates a new StringField marked as optional
func (f *StringField) Optional() *StringField {
	newField := *f
	newField.required = false
	return &newField
}

// Default creates a new StringField with a default value
func (f *StringField) Default(value string) *StringField {
	newField := *f
	newField.defaultValue = &value
	return &newField
}

// MinLength creates a new StringField with minimum length constraint
func (f *StringField) MinLength(length int) *StringField {
	newField := *f
	newField.minLength = &length
	return &newField
}

// MaxLength creates a new StringField with maximum length constraint
func (f *StringField) MaxLength(length int) *StringField {
	newField := *f
	newField.maxLength = &length
	return &newField
}

// ValidateEmail creates a new StringField with email validation
func (f *StringField) ValidateEmail() *StringField {
	newField := *f
	newField.validator = func(value string) error {
		if !IsValidEmail(value) {
			return NewAnswerError("Please enter a valid email address")
		}
		return nil
	}
	return &newField
}

// ValidateSlug creates a new StringField with slug validation
func (f *StringField) ValidateSlug() *StringField {
	newField := *f
	newField.validator = func(value string) error {
		if !IsValidSlug(value) {
			return NewAnswerError("Please enter a valid slug (lowercase letters, numbers, and hyphens only)")
		}
		return nil
	}
	return &newField
}

// ValidateCustom creates a new StringField with custom validation
func (f *StringField) ValidateCustom(validator func(string) error) *StringField {
	newField := *f
	newField.validator = validator
	return &newField
}

// ReadAnswer reads and validates the user's string answer
func (f *StringField) ReadAnswer() error {
	prompt := f.buildPrompt()
	var response string

	err := f.reader.readParamFromStdin(&response, prompt)
	if err != nil {
		return err
	}

	if response == "" {
		f.empty = true
		return nil
	}

	f.value = response
	return f.validateValue(response)
}

// validateValue validates the string value against all constraints
func (f *StringField) validateValue(value string) error {
	// Check length constraints
	if f.minLength != nil && len(value) < *f.minLength {
		return NewAnswerError(fmt.Sprintf("Must be at least %d characters long", *f.minLength))
	}

	if f.maxLength != nil && len(value) > *f.maxLength {
		return NewAnswerError(fmt.Sprintf("Must be no more than %d characters long", *f.maxLength))
	}

	// Apply custom validator
	if f.validator != nil {
		return f.validator(value)
	}

	return nil
}

// Prompt prompts the user for string input
func (f *StringField) Prompt(ctx context.Context) (any, error) {
	for {
		// Create a new field instance for each attempt to avoid state pollution
		fieldCopy := *f
		fieldCopy.empty = false
		fieldCopy.value = ""

		err := fieldCopy.ReadAnswer()
		if err != nil {
			var answerErr AnswerError
			if errors.As(err, &answerErr) {
				fmt.Fprintf(f.reader.output, "Error: %v\n", err)
				continue
			}
			return nil, err
		}

		// Handle empty response
		if fieldCopy.empty {
			if f.defaultValue != nil {
				return *f.defaultValue, nil
			}
			if f.required {
				fmt.Fprintf(f.reader.output, "Error: %v\n", NewRequiredFieldError(f.label))
				continue
			}
			return "", nil // Return empty string for optional fields
		}

		return fieldCopy.value, nil
	}
}

// buildPrompt builds the prompt string
func (f *StringField) buildPrompt() string {
	prompt := f.label

	if f.defaultValue != nil {
		prompt += fmt.Sprintf(" [default: %s]", *f.defaultValue)
	}

	return prompt + ": "
}
