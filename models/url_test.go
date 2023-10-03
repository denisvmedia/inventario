package models_test

import (
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestPositiveURLValidation(t *testing.T) {
	testCases := []struct {
		url *models.URL
	}{
		{url: &models.URL{Scheme: "http", Host: "example.com"}},
	}

	for _, tc := range testCases {
		tc := tc // Capture the range variable.
		t.Run(tc.url.String(), func(t *testing.T) {
			c := qt.New(t)

			err := tc.url.Validate()

			c.Assert(err, qt.IsNil)
		})
	}
}

func TestNegativeURLValidation(t *testing.T) {
	testCases := []struct {
		url      *models.URL
		errorMsg string
	}{
		{url: &models.URL{Scheme: "invalid", Host: "example.com"}, errorMsg: "validation error message"},
	}

	for _, tc := range testCases {
		tc := tc // Capture the range variable.
		t.Run(tc.url.String(), func(t *testing.T) {
			c := qt.New(t)

			err := tc.url.Validate()
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Equals, "Scheme: must be a valid value.")
		})
	}
}

func TestPositiveURLJSONMarshalling(t *testing.T) {
	testCases := []struct {
		url      *models.URL
		expected string
	}{
		{url: &models.URL{Scheme: "http", Host: "example.com"}, expected: `"http://example.com"`},
		// Add more positive test cases here.
	}

	for _, tc := range testCases {
		tc := tc // Capture the range variable.
		t.Run(tc.expected, func(t *testing.T) {
			c := qt.New(t)

			data, err := json.Marshal(tc.url)

			c.Assert(err, qt.IsNil)
			c.Assert(string(data), qt.Equals, tc.expected, qt.Commentf("Expected JSON output: %s", tc.expected))
		})
	}
}
