package rules_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

// mockRule is a mock implementation of validation.Rule for testing
type mockRule struct {
	err error
}

func (r mockRule) Validate(any) error {
	return r.err
}

func TestWhenTrueRule_Validate(t *testing.T) {
	// Happy path tests
	t.Run("when value is false, rules are not applied", func(t *testing.T) {
		c := qt.New(t)
		// Create a rule that would fail if applied
		mockErr := validation.NewError("mock_error", "mock error")
		mockRule := mockRule{err: mockErr}

		rule := rules.WhenTrue(false, mockRule)
		err := rule.Validate(nil)
		c.Assert(err, qt.IsNil)
	})

	t.Run("when value is true and rules pass, validation passes", func(t *testing.T) {
		c := qt.New(t)
		// Create a rule that passes
		mockRule := mockRule{err: nil}

		rule := rules.WhenTrue(true, mockRule)
		err := rule.Validate(nil)
		c.Assert(err, qt.IsNil)
	})

	// Unhappy path tests
	t.Run("when value is true and rules fail, validation fails", func(t *testing.T) {
		c := qt.New(t)
		// Create a rule that fails
		mockErr := validation.NewError("mock_error", "mock error")
		mockRule := mockRule{err: mockErr}

		rule := rules.WhenTrue(true, mockRule)
		err := rule.Validate(nil)
		c.Assert(err, qt.IsNotNil)

		// Check that the error message contains our mock error message
		c.Assert(err.Error(), qt.Contains, mockErr.Error())
	})

	t.Run("when value is true and multiple rules fail, all errors are returned", func(t *testing.T) {
		c := qt.New(t)
		// Create rules that fail with different errors
		mockErr1 := validation.NewError("mock_error_1", "mock error 1")
		mockErr2 := validation.NewError("mock_error_2", "mock error 2")
		mockRule1 := mockRule{err: mockErr1}
		mockRule2 := mockRule{err: mockErr2}

		rule := rules.WhenTrue(true, mockRule1, mockRule2)
		err := rule.Validate(nil)
		c.Assert(err, qt.IsNotNil)

		// Check that the error message contains both mock error messages
		c.Assert(err.Error(), qt.Contains, mockErr1.Error())
		c.Assert(err.Error(), qt.Contains, mockErr2.Error())
	})
}

func TestWhenTrueRule_WithRules(t *testing.T) {
	t.Run("adds rules to existing rule", func(t *testing.T) {
		c := qt.New(t)
		// Create initial rule with one mock rule
		mockErr1 := validation.NewError("mock_error_1", "mock error 1")
		mockRule1 := mockRule{err: mockErr1}
		initialRule := rules.WhenTrue(true, mockRule1)

		// Add another rule
		mockErr2 := validation.NewError("mock_error_2", "mock error 2")
		mockRule2 := mockRule{err: mockErr2}
		updatedRule := initialRule.WithRules(mockRule2)

		// Validate and check that both errors are present
		err := updatedRule.Validate(nil)
		c.Assert(err, qt.IsNotNil)

		// Check that the error message contains both mock error messages
		c.Assert(err.Error(), qt.Contains, mockErr1.Error())
		c.Assert(err.Error(), qt.Contains, mockErr2.Error())
	})
}
