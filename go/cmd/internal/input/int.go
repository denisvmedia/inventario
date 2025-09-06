package input

import (
	"context"
	"errors"
	"fmt"
)

// IntField represents an integer input field
type IntField struct {
	label        string
	reader       *Reader
	required     bool
	defaultValue *int
	minValue     *int
	maxValue     *int
	empty        bool
	value        int
}

// NewIntField creates a new integer input field
func NewIntField(label string, reader *Reader) *IntField {
	return &IntField{
		label:  label,
		reader: reader,
	}
}

// SetReader sets the reader for the field (used for testing)
func (f *IntField) SetReader(reader *Reader) {
	f.reader = reader
}

// Required creates a new IntField marked as required
func (f *IntField) Required() *IntField {
	newField := *f
	newField.required = true
	return &newField
}

// Optional creates a new IntField marked as optional
func (f *IntField) Optional() *IntField {
	newField := *f
	newField.required = false
	return &newField
}

// Default creates a new IntField with a default value
func (f *IntField) Default(value int) *IntField {
	newField := *f
	newField.defaultValue = &value
	return &newField
}

// Min creates a new IntField with minimum value constraint
func (f *IntField) Min(value int) *IntField {
	newField := *f
	newField.minValue = &value
	return &newField
}

// Max creates a new IntField with maximum value constraint
func (f *IntField) Max(value int) *IntField {
	newField := *f
	newField.maxValue = &value
	return &newField
}

// Range creates a new IntField with min and max value constraints
func (f *IntField) Range(minVal, maxVal int) *IntField {
	newField := *f
	newField.minValue = &minVal
	newField.maxValue = &maxVal
	return &newField
}

// Positive creates a new IntField that only accepts positive values
func (f *IntField) Positive() *IntField {
	newField := *f
	minVal := 1
	newField.minValue = &minVal
	return &newField
}

// NonNegative creates a new IntField that only accepts non-negative values
func (f *IntField) NonNegative() *IntField {
	newField := *f
	minVal := 0
	newField.minValue = &minVal
	return &newField
}

// ReadAnswer reads and validates the user's integer answer
func (f *IntField) ReadAnswer() error {
	prompt := f.buildPrompt()
	var response int

	err := f.reader.readIntFromStdin(&response, prompt)
	if err != nil {
		// Check if it's an empty response
		var answerErr AnswerError
		if errors.As(err, &answerErr) && answerErr.Message == "" {
			f.empty = true
			return nil
		}
		return err
	}

	f.value = response
	return f.validateValue(response)
}

// validateValue validates the integer value against all constraints
func (f *IntField) validateValue(value int) error {
	// Check minimum value
	if f.minValue != nil && value < *f.minValue {
		return NewAnswerError(fmt.Sprintf("Value must be at least %d", *f.minValue))
	}

	// Check maximum value
	if f.maxValue != nil && value > *f.maxValue {
		return NewAnswerError(fmt.Sprintf("Value must be no more than %d", *f.maxValue))
	}

	return nil
}

// Prompt prompts the user for integer input
func (f *IntField) Prompt(ctx context.Context) (any, error) {
	for {
		// Create a new field instance for each attempt to avoid state pollution
		fieldCopy := *f
		fieldCopy.empty = false
		fieldCopy.value = 0

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
			return 0, nil // Return zero for optional fields
		}

		return fieldCopy.value, nil
	}
}

// buildPrompt builds the prompt string
func (f *IntField) buildPrompt() string {
	prompt := f.label

	if f.defaultValue != nil {
		prompt += fmt.Sprintf(" [default: %d]", *f.defaultValue)
	}

	switch {
	case f.minValue != nil && f.maxValue != nil:
		prompt += fmt.Sprintf(" (%d-%d)", *f.minValue, *f.maxValue)
	case f.minValue != nil:
		prompt += fmt.Sprintf(" (min: %d)", *f.minValue)
	case f.maxValue != nil:
		prompt += fmt.Sprintf(" (max: %d)", *f.maxValue)
	}

	return prompt + ": "
}
