package models_test

import (
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jellydator/validation"

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

			var validationErrors validation.Errors
			c.Assert(err, qt.ErrorAs, &validationErrors)
			c.Assert(validationErrors, qt.HasLen, 1)

			var validationError validation.ErrorObject
			c.Assert(validationErrors["Scheme"], qt.ErrorAs, &validationError)
			c.Assert(validationError.Code(), qt.Equals, "validation_in_invalid")
			c.Assert(validationError.Message(), qt.Equals, "must be a valid value")
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

func TestURLsJSON(t *testing.T) {
	c := qt.New(t)

	data := `"http://example.com\nhttp://example.org\nhttp://example.net"`

	var u models.URLs
	err := json.Unmarshal([]byte(data), &u)

	c.Assert(err, qt.IsNil)
	c.Assert(u, qt.HasLen, 3)
	c.Assert(u[0].String(), qt.Equals, "http://example.com")
	c.Assert(u[1].String(), qt.Equals, "http://example.org")
	c.Assert(u[2].String(), qt.Equals, "http://example.net")

	marshaled, err := json.Marshal(&u)
	c.Assert(err, qt.IsNil)
	c.Assert(string(marshaled), qt.Equals, data)
}
