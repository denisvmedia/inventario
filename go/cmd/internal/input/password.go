package input

import (
	"context"
	"errors"
	"fmt"
)

// PasswordField represents a password input field
type PasswordField struct {
	label            string
	reader           *Reader
	required         bool
	withConfirmation bool
	minLength        *int
	validator        func(string) error
	value            string
}

// NewPasswordField creates a new password input field
func NewPasswordField(label string, reader *Reader) *PasswordField {
	return &PasswordField{
		label:            label,
		reader:           reader,
		required:         true, // Passwords are required by default
		withConfirmation: true, // Confirmation is enabled by default
	}
}

// SetReader sets the reader for the field (used for testing)
func (f *PasswordField) SetReader(reader *Reader) {
	f.reader = reader
}

// Required creates a new PasswordField marked as required
func (f *PasswordField) Required() *PasswordField {
	newField := *f
	newField.required = true
	return &newField
}

// Optional creates a new PasswordField marked as optional
func (f *PasswordField) Optional() *PasswordField {
	newField := *f
	newField.required = false
	return &newField
}

// WithConfirmation creates a new PasswordField with confirmation prompt
func (f *PasswordField) WithConfirmation() *PasswordField {
	newField := *f
	newField.withConfirmation = true
	return &newField
}

// NoConfirmation creates a new PasswordField without confirmation prompt
func (f *PasswordField) NoConfirmation() *PasswordField {
	newField := *f
	newField.withConfirmation = false
	return &newField
}

// MinLength creates a new PasswordField with minimum length constraint
func (f *PasswordField) MinLength(length int) *PasswordField {
	newField := *f
	newField.minLength = &length
	return &newField
}

// ValidateStrength creates a new PasswordField with strength validation
func (f *PasswordField) ValidateStrength() *PasswordField {
	newField := *f
	newField.validator = func(password string) error {
		if !IsValidPassword(password) {
			return NewAnswerError("Password must be at least 8 characters long and contain uppercase, lowercase, and digit")
		}
		return nil
	}
	return &newField
}

// ValidateCustom creates a new PasswordField with custom validation
func (f *PasswordField) ValidateCustom(validator func(string) error) *PasswordField {
	newField := *f
	newField.validator = validator
	return &newField
}

// ReadAnswer reads and validates the user's password
func (f *PasswordField) ReadAnswer() error {
	// Read password
	password, err := f.reader.readPasswordFromStdin(f.label + ": ")
	if err != nil {
		return err
	}

	if password == "" && f.required {
		return NewRequiredFieldError(f.label)
	}

	// Validate password
	if password != "" {
		if err := f.validateValue(password); err != nil {
			return err
		}
	}

	// Read confirmation if required
	if f.withConfirmation && password != "" {
		confirmation, err := f.reader.readPasswordFromStdin("Confirm " + f.label + ": ")
		if err != nil {
			return err
		}

		if password != confirmation {
			return NewAnswerError("Passwords do not match")
		}
	}

	f.value = password
	return nil
}

// validateValue validates the password against all constraints
func (f *PasswordField) validateValue(password string) error {
	// Check minimum length
	if f.minLength != nil && len(password) < *f.minLength {
		return NewAnswerError(fmt.Sprintf("Password must be at least %d characters long", *f.minLength))
	}

	// Apply custom validator
	if f.validator != nil {
		return f.validator(password)
	}

	return nil
}

// Prompt prompts the user for password input
func (f *PasswordField) Prompt(ctx context.Context) (any, error) {
	for {
		// Create a new field instance for each attempt to avoid state pollution
		fieldCopy := *f
		fieldCopy.value = ""

		err := fieldCopy.ReadAnswer()
		if err != nil {
			var answerErr AnswerError
			var requiredErr RequiredFieldError
			if errors.As(err, &answerErr) || errors.As(err, &requiredErr) {
				fmt.Fprintf(f.reader.output, "Error: %v\n", err)
				continue
			}
			return nil, err
		}

		return fieldCopy.value, nil
	}
}
