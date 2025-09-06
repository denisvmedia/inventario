package input

import (
	"context"
	"fmt"
	"io"
	"os"
)

// Form provides a simple way to collect multiple inputs
type Form struct {
	reader *Reader
	fields []InputField
}

// NewForm creates a new form with the specified input and output
func NewForm(input io.Reader, output io.Writer) *Form {
	return &Form{
		reader: NewReader(input, output),
		fields: make([]InputField, 0),
	}
}

// NewDefaultForm creates a form with stdin/stdout
func NewDefaultForm() *Form {
	return NewForm(os.Stdin, os.Stdout)
}

// AddBool adds a boolean field to the form
func (f *Form) AddBool(label string) *BoolField {
	field := NewBoolField(label, f.reader)
	f.fields = append(f.fields, field)
	return field
}

// AddString adds a string field to the form
func (f *Form) AddString(label string) *StringField {
	field := NewStringField(label, f.reader)
	f.fields = append(f.fields, field)
	return field
}

// AddPassword adds a password field to the form
func (f *Form) AddPassword(label string) *PasswordField {
	field := NewPasswordField(label, f.reader)
	f.fields = append(f.fields, field)
	return field
}

// AddInt adds an integer field to the form
func (f *Form) AddInt(label string) *IntField {
	field := NewIntField(label, f.reader)
	f.fields = append(f.fields, field)
	return field
}

// Collect collects all field values and returns them as a map
func (f *Form) Collect(ctx context.Context) (map[string]any, error) {
	results := make(map[string]any)

	for _, field := range f.fields {
		value, err := field.Prompt(ctx)
		if err != nil {
			return nil, err
		}

		// Extract field name based on field type
		var fieldName string
		switch v := field.(type) {
		case *BoolField:
			fieldName = v.label
		case *StringField:
			fieldName = v.label
		case *PasswordField:
			fieldName = v.label
		case *IntField:
			fieldName = v.label
		}

		results[fieldName] = value
	}

	return results, nil
}

// Quick provides simple one-off input collection functions
type Quick struct {
	reader *Reader
}

// NewQuick creates a new Quick instance with the specified input and output
func NewQuick(input io.Reader, output io.Writer) *Quick {
	return &Quick{
		reader: NewReader(input, output),
	}
}

// NewDefaultQuick creates a Quick instance with stdin/stdout
func NewDefaultQuick() *Quick {
	return NewQuick(os.Stdin, os.Stdout)
}

// Bool prompts for a boolean value
func (q *Quick) Bool(ctx context.Context, label string) (bool, error) {
	field := NewBoolField(label, q.reader)
	value, err := field.Prompt(ctx)
	if err != nil {
		return false, err
	}
	result, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("unexpected type returned from boolean field")
	}
	return result, nil
}

// String prompts for a string value
func (q *Quick) String(ctx context.Context, label string) (string, error) {
	field := NewStringField(label, q.reader)
	value, err := field.Prompt(ctx)
	if err != nil {
		return "", err
	}
	result, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("unexpected type returned from string field")
	}
	return result, nil
}

// Password prompts for a password value
func (q *Quick) Password(ctx context.Context, label string) (string, error) {
	field := NewPasswordField(label, q.reader)
	value, err := field.Prompt(ctx)
	if err != nil {
		return "", err
	}
	result, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("unexpected type returned from password field")
	}
	return result, nil
}

// Int prompts for an integer value
func (q *Quick) Int(ctx context.Context, label string) (int, error) {
	field := NewIntField(label, q.reader)
	value, err := field.Prompt(ctx)
	if err != nil {
		return 0, err
	}
	result, ok := value.(int)
	if !ok {
		return 0, fmt.Errorf("unexpected type returned from integer field")
	}
	return result, nil
}

// Email prompts for an email address
func (q *Quick) Email(ctx context.Context, label string) (string, error) {
	field := NewStringField(label, q.reader).ValidateEmail()
	value, err := field.Prompt(ctx)
	if err != nil {
		return "", err
	}
	result, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("unexpected type returned from email field")
	}
	return result, nil
}

// Slug prompts for a slug value
func (q *Quick) Slug(ctx context.Context, label string) (string, error) {
	field := NewStringField(label, q.reader).ValidateSlug()
	value, err := field.Prompt(ctx)
	if err != nil {
		return "", err
	}
	result, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("unexpected type returned from slug field")
	}
	return result, nil
}
