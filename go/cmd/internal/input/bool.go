package input

import (
	"context"
	"errors"
	"fmt"
)

// BoolVal represents a boolean value with display formatting
type BoolVal bool
type PBoolVal = *BoolVal

// String returns the formatted display string for the boolean value
func (b *BoolVal) String() string {
	if b == nil {
		return "y/n"
	}
	if *b {
		return "Y/n"
	}
	return "y/N"
}

// BoolField represents a boolean input field
type BoolField struct {
	label        string
	reader       *Reader
	required     bool
	defaultValue *bool
	empty        bool
	value        bool
}

// NewBoolField creates a new boolean input field
func NewBoolField(label string, reader *Reader) *BoolField {
	return &BoolField{
		label:  label,
		reader: reader,
	}
}

// SetReader sets the reader for the field (used for testing)
func (f *BoolField) SetReader(reader *Reader) {
	f.reader = reader
}

// Required creates a new BoolField marked as required
func (f *BoolField) Required() *BoolField {
	newField := *f
	newField.required = true
	return &newField
}

// Optional creates a new BoolField marked as optional
func (f *BoolField) Optional() *BoolField {
	newField := *f
	newField.required = false
	return &newField
}

// DefaultYes creates a new BoolField with default value true
func (f *BoolField) DefaultYes() *BoolField {
	newField := *f
	defaultVal := true
	newField.defaultValue = &defaultVal
	return &newField
}

// DefaultNo creates a new BoolField with default value false
func (f *BoolField) DefaultNo() *BoolField {
	newField := *f
	defaultVal := false
	newField.defaultValue = &defaultVal
	return &newField
}

// ReadAnswer reads and parses the user's boolean answer
func (f *BoolField) ReadAnswer() error {
	prompt := f.buildPrompt()
	var response string

	err := f.reader.readParamFromStdin(&response, prompt)
	if err != nil {
		return err
	}

	switch response {
	case "":
		f.empty = true
	case "Y", "y", "yes", "Yes", "YES":
		f.value = true
	case "N", "n", "no", "No", "NO":
		f.value = false
	default:
		return NewAnswerError("Choose y or n")
	}

	return nil
}

// Prompt prompts the user for boolean input
func (f *BoolField) Prompt(ctx context.Context) (any, error) {
	for {
		// Create a new field instance for each attempt to avoid state pollution
		fieldCopy := *f
		fieldCopy.empty = false
		fieldCopy.value = false

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
			return false, nil // Default to false for optional fields
		}

		return fieldCopy.value, nil
	}
}

// buildPrompt builds the prompt string
func (f *BoolField) buildPrompt() string {
	prompt := f.label
	boolVal := PBoolVal(f.defaultValue)
	prompt += " (" + boolVal.String() + ")"

	return prompt + ": "
}
