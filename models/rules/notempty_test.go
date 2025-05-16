package rules_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models/rules"
)

func TestNotEmpty_Validate(t *testing.T) {
	// Happy path tests
	t.Run("valid non-empty values", func(t *testing.T) {
		testCases := []struct {
			name  string
			value any
		}{
			{
				name:  "non-empty string",
				value: "test",
			},
			{
				name:  "non-empty array",
				value: [1]int{1},
			},
			{
				name:  "non-empty slice",
				value: []int{1, 2, 3},
			},
			{
				name:  "non-empty map",
				value: map[string]int{"key": 1},
			},
			{
				name:  "non-nil pointer",
				value: &struct{}{},
			},
			{
				name:  "integer",
				value: 42,
			},
			{
				name:  "boolean",
				value: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				c := qt.New(t)
				err := rules.NotEmpty.Validate(tc.value)
				c.Assert(err, qt.IsNil)
			})
		}
	})

	// Unhappy path tests
	t.Run("invalid empty values", func(t *testing.T) {
		testCases := []struct {
			name  string
			value any
		}{
			{
				name:  "nil",
				value: nil,
			},
			{
				name:  "empty string",
				value: "",
			},
			{
				name:  "whitespace string",
				value: "   ",
			},
			{
				name:  "empty array",
				value: [0]int{},
			},
			{
				name:  "empty slice",
				value: make([]int, 0),
			},
			{
				name:  "empty map",
				value: make(map[string]int),
			},
			{
				name:  "nil pointer",
				value: (*struct{})(nil),
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				c := qt.New(t)
				err := rules.NotEmpty.Validate(tc.value)
				c.Assert(err, qt.IsNotNil)
				c.Assert(err.Error(), qt.Equals, rules.ErrIsEmpty.Error())
			})
		}
	})
}
