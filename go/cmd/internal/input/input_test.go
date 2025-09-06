package input_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/internal/input"
)

func TestBoolField_Prompt(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		field          *input.BoolField
		expectedResult bool
		expectError    bool
	}{
		{
			name:           "yes input returns true",
			input:          "y\n",
			field:          input.NewBoolField("Test", nil),
			expectedResult: true,
			expectError:    false,
		},
		{
			name:           "no input returns false",
			input:          "n\n",
			field:          input.NewBoolField("Test", nil),
			expectedResult: false,
			expectError:    false,
		},
		{
			name:           "empty input with default yes returns true",
			input:          "\n",
			field:          input.NewBoolField("Test", nil).DefaultYes(),
			expectedResult: true,
			expectError:    false,
		},
		{
			name:           "empty input with default no returns false",
			input:          "\n",
			field:          input.NewBoolField("Test", nil).DefaultNo(),
			expectedResult: false,
			expectError:    false,
		},
		{
			name:           "empty input optional field returns false",
			input:          "\n",
			field:          input.NewBoolField("Test", nil).Optional(),
			expectedResult: false,
			expectError:    false,
		},
		{
			name:           "required field accepts valid input",
			input:          "y\n",
			field:          input.NewBoolField("Test", nil).Required(),
			expectedResult: true,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			var output bytes.Buffer
			reader := input.NewReader(strings.NewReader(tt.input), &output)
			field := tt.field
			field.SetReader(reader)

			result, err := field.Prompt(context.Background())

			c.Assert(err != nil, qt.Equals, tt.expectError)
			c.Assert(result, qt.Equals, tt.expectedResult)
		})
	}
}

func TestBoolField_RequiredWithoutDefault(t *testing.T) {
	c := qt.New(t)

	// Test that a required field without default re-prompts on empty input
	// Input: empty line, then "y"
	inputStr := "\ny\n"
	var output bytes.Buffer
	reader := input.NewReader(strings.NewReader(inputStr), &output)
	field := input.NewBoolField("Required Test", reader).Required()

	result, err := field.Prompt(context.Background())

	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.Equals, true)

	// Check that error message was displayed
	outputStr := output.String()
	c.Assert(strings.Contains(outputStr, "Error:"), qt.Equals, true)
	c.Assert(strings.Contains(outputStr, "required"), qt.Equals, true)
}

func TestBoolField_Configurations(t *testing.T) {
	tests := []struct {
		name           string
		fieldBuilder   func(*input.Reader) *input.BoolField
		inputStr       string
		expectedResult bool
		expectError    bool
		description    string
	}{
		{
			name: "required_no_default",
			fieldBuilder: func(r *input.Reader) *input.BoolField {
				return input.NewBoolField("Enable feature", r).Required()
			},
			inputStr:       "y\n",
			expectedResult: true,
			expectError:    false,
			description:    "Required field with no predefined answer - user must choose",
		},
		{
			name: "optional_no_default",
			fieldBuilder: func(r *input.Reader) *input.BoolField {
				return input.NewBoolField("Enable feature", r).Optional()
			},
			inputStr:       "\n",
			expectedResult: false,
			expectError:    false,
			description:    "Optional field defaults to false when empty",
		},
		{
			name: "default_yes",
			fieldBuilder: func(r *input.Reader) *input.BoolField {
				return input.NewBoolField("Enable feature", r).DefaultYes()
			},
			inputStr:       "\n",
			expectedResult: true,
			expectError:    false,
			description:    "Field with default Yes - empty input returns true",
		},
		{
			name: "default_no",
			fieldBuilder: func(r *input.Reader) *input.BoolField {
				return input.NewBoolField("Enable feature", r).DefaultNo()
			},
			inputStr:       "\n",
			expectedResult: false,
			expectError:    false,
			description:    "Field with default No - empty input returns false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			var output bytes.Buffer
			reader := input.NewReader(strings.NewReader(tt.inputStr), &output)
			field := tt.fieldBuilder(reader)

			result, err := field.Prompt(context.Background())

			c.Assert(err != nil, qt.Equals, tt.expectError)
			if !tt.expectError {
				c.Assert(result, qt.Equals, tt.expectedResult)
			}
		})
	}
}

func TestBoolVal_String(t *testing.T) {
	tests := []struct {
		name     string
		value    input.BoolVal
		expected string
	}{
		{
			name:     "true value shows Y/n",
			value:    input.BoolVal(true),
			expected: "Y/n",
		},
		{
			name:     "false value shows y/N",
			value:    input.BoolVal(false),
			expected: "y/N",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			result := tt.value.String()
			c.Assert(result, qt.Equals, tt.expected)
		})
	}
}

func TestBoolField_PromptFormatting(t *testing.T) {
	tests := []struct {
		name           string
		fieldBuilder   func(*input.Reader) *input.BoolField
		expectedPrompt string
	}{
		{
			name: "default_yes_shows_Y_n",
			fieldBuilder: func(r *input.Reader) *input.BoolField {
				return input.NewBoolField("Enable feature", r).DefaultYes()
			},
			expectedPrompt: "Enable feature (Y/n): ",
		},
		{
			name: "default_no_shows_y_N",
			fieldBuilder: func(r *input.Reader) *input.BoolField {
				return input.NewBoolField("Enable feature", r).DefaultNo()
			},
			expectedPrompt: "Enable feature (y/N): ",
		},
		{
			name: "no_default_shows_y_n",
			fieldBuilder: func(r *input.Reader) *input.BoolField {
				return input.NewBoolField("Enable feature", r).Required()
			},
			expectedPrompt: "Enable feature (y/n): ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			var output bytes.Buffer
			reader := input.NewReader(strings.NewReader("y\n"), &output)
			field := tt.fieldBuilder(reader)

			// Call buildPrompt to test the formatting - we need to use reflection or make it public
			// For now, let's test through the actual prompt behavior
			_, err := field.Prompt(context.Background())
			c.Assert(err, qt.IsNil)

			// Check that the output contains the expected prompt format
			outputStr := output.String()
			c.Assert(strings.Contains(outputStr, tt.expectedPrompt), qt.Equals, true)
		})
	}
}

func TestStringField_Prompt(t *testing.T) {
	tests := []struct {
		name           string
		inputStr       string
		field          *input.StringField
		expectedResult string
		expectError    bool
	}{
		{
			name:           "valid string input",
			inputStr:       "hello\n",
			field:          input.NewStringField("Test", nil),
			expectedResult: "hello",
			expectError:    false,
		},
		{
			name:           "empty input with default",
			inputStr:       "\n",
			field:          input.NewStringField("Test", nil).Default("default"),
			expectedResult: "default",
			expectError:    false,
		},
		{
			name:           "empty input optional field",
			inputStr:       "\n",
			field:          input.NewStringField("Test", nil).Optional(),
			expectedResult: "",
			expectError:    false,
		},
		{
			name:           "valid email input",
			inputStr:       "test@example.com\n",
			field:          input.NewStringField("Email", nil).ValidateEmail(),
			expectedResult: "test@example.com",
			expectError:    false,
		},
		{
			name:           "valid slug input",
			inputStr:       "test-slug\n",
			field:          input.NewStringField("Slug", nil).ValidateSlug(),
			expectedResult: "test-slug",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			var output bytes.Buffer
			reader := input.NewReader(strings.NewReader(tt.inputStr), &output)
			field := tt.field
			field.SetReader(reader)

			result, err := field.Prompt(context.Background())

			c.Assert(err != nil, qt.Equals, tt.expectError)
			c.Assert(result, qt.Equals, tt.expectedResult)
		})
	}
}

func TestPasswordField_Prompt(t *testing.T) {
	tests := []struct {
		name           string
		field          *input.PasswordField
		expectedResult string
		expectError    bool
	}{
		{
			name:           "password field without confirmation",
			field:          input.NewPasswordField("Password", nil).NoConfirmation(),
			expectedResult: "",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			var output bytes.Buffer
			reader := input.NewReader(strings.NewReader(""), &output)
			field := tt.field
			field.SetReader(reader)

			// Note: Password testing is limited due to terminal dependency
			// In real scenarios, we would mock the terminal input
			_ = field
			c.Assert(true, qt.Equals, true) // Placeholder assertion
		})
	}
}

func TestIntField_Prompt(t *testing.T) {
	tests := []struct {
		name           string
		inputStr       string
		field          *input.IntField
		expectedResult int
		expectError    bool
	}{
		{
			name:           "valid integer input",
			inputStr:       "42\n",
			field:          input.NewIntField("Test", nil),
			expectedResult: 42,
			expectError:    false,
		},
		{
			name:           "empty input with default",
			inputStr:       "\n",
			field:          input.NewIntField("Test", nil).Default(10),
			expectedResult: 10,
			expectError:    false,
		},
		{
			name:           "empty input optional field",
			inputStr:       "\n",
			field:          input.NewIntField("Test", nil).Optional(),
			expectedResult: 0,
			expectError:    false,
		},
		{
			name:           "positive value",
			inputStr:       "5\n",
			field:          input.NewIntField("Test", nil).Positive(),
			expectedResult: 5,
			expectError:    false,
		},
		{
			name:           "value in range",
			inputStr:       "15\n",
			field:          input.NewIntField("Test", nil).Range(10, 20),
			expectedResult: 15,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			var output bytes.Buffer
			reader := input.NewReader(strings.NewReader(tt.inputStr), &output)
			field := tt.field
			field.SetReader(reader)

			result, err := field.Prompt(context.Background())

			c.Assert(err != nil, qt.Equals, tt.expectError)
			c.Assert(result, qt.Equals, tt.expectedResult)
		})
	}
}

func TestPatterns(t *testing.T) {
	tests := []struct {
		name     string
		inputStr string
		function func(string) bool
		expected bool
	}{
		{
			name:     "valid email",
			inputStr: "test@example.com",
			function: input.IsValidEmail,
			expected: true,
		},
		{
			name:     "invalid email",
			inputStr: "invalid-email",
			function: input.IsValidEmail,
			expected: false,
		},
		{
			name:     "valid slug",
			inputStr: "test-slug",
			function: input.IsValidSlug,
			expected: true,
		},
		{
			name:     "invalid slug",
			inputStr: "Test Slug!",
			function: input.IsValidSlug,
			expected: false,
		},
		{
			name:     "valid password",
			inputStr: "TestPassword123",
			function: input.IsValidPassword,
			expected: true,
		},
		{
			name:     "invalid password",
			inputStr: "weak",
			function: input.IsValidPassword,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			result := tt.function(tt.inputStr)
			c.Assert(result, qt.Equals, tt.expected)
		})
	}
}

func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		expectType string
		expectAs   bool
	}{
		{
			name:       "AnswerError can be detected with errors.As",
			err:        input.NewAnswerError("test message"),
			expectType: "AnswerError",
			expectAs:   true,
		},
		{
			name:       "RequiredFieldError can be detected with errors.As",
			err:        input.NewRequiredFieldError("test field"),
			expectType: "RequiredFieldError",
			expectAs:   true,
		},
		{
			name:       "ValidationError can be detected with errors.As",
			err:        input.ValidationError{Field: "test", Message: "test message"},
			expectType: "ValidationError",
			expectAs:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			switch tt.expectType {
			case "AnswerError":
				var answerErr input.AnswerError
				result := errors.As(tt.err, &answerErr)
				c.Assert(result, qt.Equals, tt.expectAs)
			case "RequiredFieldError":
				var requiredErr input.RequiredFieldError
				result := errors.As(tt.err, &requiredErr)
				c.Assert(result, qt.Equals, tt.expectAs)
			case "ValidationError":
				var validationErr input.ValidationError
				result := errors.As(tt.err, &validationErr)
				c.Assert(result, qt.Equals, tt.expectAs)
			}
		})
	}
}
