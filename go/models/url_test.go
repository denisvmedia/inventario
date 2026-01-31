package models_test

import (
	"context"
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestURLParse(t *testing.T) {
	t.Run("valid URL", func(t *testing.T) {
		c := qt.New(t)

		urlStr := "https://example.com/path?query=value#fragment"
		parsedURL, err := models.URLParse(urlStr)

		c.Assert(err, qt.IsNil)
		c.Assert(parsedURL, qt.IsNotNil)
		c.Assert(parsedURL.Scheme, qt.Equals, "https")
		c.Assert(parsedURL.Host, qt.Equals, "example.com")
		c.Assert(parsedURL.Path, qt.Equals, "/path")
		c.Assert(parsedURL.RawQuery, qt.Equals, "query=value")
		c.Assert(parsedURL.Fragment, qt.Equals, "fragment")
	})

	t.Run("invalid URL", func(t *testing.T) {
		c := qt.New(t)

		urlStr := "://invalid-url"
		parsedURL, err := models.URLParse(urlStr)

		c.Assert(err, qt.IsNotNil)
		c.Assert(parsedURL, qt.IsNil)
	})
}

func TestURL_String(t *testing.T) {
	c := qt.New(t)

	// Create a URL
	url := &models.URL{
		Scheme:   "https",
		Host:     "example.com",
		Path:     "/path",
		RawQuery: "query=value",
		Fragment: "fragment",
	}

	// Test String method
	c.Assert(url.String(), qt.Equals, "https://example.com/path?query=value#fragment")
}

func TestURL_Validate(t *testing.T) {
	c := qt.New(t)

	url := &models.URL{}
	err := url.Validate()
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Equals, "must use validate with context")
}

func TestURL_ValidateWithContext_HappyPath(t *testing.T) {
	testCases := []struct {
		name string
		url  *models.URL
	}{
		{
			name: "http URL",
			url:  &models.URL{Scheme: "http", Host: "example.com"},
		},
		{
			name: "https URL",
			url:  &models.URL{Scheme: "https", Host: "example.com"},
		},
		{
			name: "URL with path",
			url:  &models.URL{Scheme: "https", Host: "example.com", Path: "/path"},
		},
		{
			name: "URL with query",
			url:  &models.URL{Scheme: "https", Host: "example.com", RawQuery: "query=value"},
		},
		{
			name: "URL with fragment",
			url:  &models.URL{Scheme: "https", Host: "example.com", Fragment: "fragment"},
		},
		{
			name: "complete URL",
			url:  &models.URL{Scheme: "https", Host: "example.com", Path: "/path", RawQuery: "query=value", Fragment: "fragment"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			ctx := context.Background()
			err := tc.url.ValidateWithContext(ctx)
			c.Assert(err, qt.IsNil)
		})
	}
}

func TestURL_ValidateWithContext_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name          string
		url           *models.URL
		errorContains string
	}{
		{
			name:          "missing scheme",
			url:           &models.URL{Host: "example.com"},
			errorContains: "Scheme: cannot be blank",
		},
		{
			name:          "missing host",
			url:           &models.URL{Scheme: "https"},
			errorContains: "Host: cannot be blank",
		},
		{
			name:          "invalid scheme",
			url:           &models.URL{Scheme: "ftp", Host: "example.com"},
			errorContains: "Scheme: must be a valid value",
		},
		{
			name:          "empty URL",
			url:           &models.URL{},
			errorContains: "Host: cannot be blank",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			ctx := context.Background()
			err := tc.url.ValidateWithContext(ctx)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.errorContains)
		})
	}
}

func TestURL_JSONMarshaling(t *testing.T) {
	testCases := []struct {
		name     string
		url      *models.URL
		expected string
	}{
		{
			name:     "http URL",
			url:      &models.URL{Scheme: "http", Host: "example.com"},
			expected: `"http://example.com"`,
		},
		{
			name:     "https URL",
			url:      &models.URL{Scheme: "https", Host: "example.com"},
			expected: `"https://example.com"`,
		},
		{
			name:     "URL with path",
			url:      &models.URL{Scheme: "https", Host: "example.com", Path: "/path"},
			expected: `"https://example.com/path"`,
		},
		{
			name:     "URL with query",
			url:      &models.URL{Scheme: "https", Host: "example.com", RawQuery: "query=value"},
			expected: `"https://example.com?query=value"`,
		},
		{
			name:     "URL with fragment",
			url:      &models.URL{Scheme: "https", Host: "example.com", Fragment: "fragment"},
			expected: `"https://example.com#fragment"`,
		},
		{
			name:     "complete URL",
			url:      &models.URL{Scheme: "https", Host: "example.com", Path: "/path", RawQuery: "query=value", Fragment: "fragment"},
			expected: `"https://example.com/path?query=value#fragment"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			// Marshal to JSON
			data, err := json.Marshal(tc.url)
			c.Assert(err, qt.IsNil)
			c.Assert(string(data), qt.Equals, tc.expected)

			// Unmarshal back to URL
			var unmarshaledURL models.URL
			err = json.Unmarshal(data, &unmarshaledURL)
			c.Assert(err, qt.IsNil)

			// Verify fields match
			c.Assert(unmarshaledURL.Scheme, qt.Equals, tc.url.Scheme)
			c.Assert(unmarshaledURL.Host, qt.Equals, tc.url.Host)
			c.Assert(unmarshaledURL.Path, qt.Equals, tc.url.Path)
			c.Assert(unmarshaledURL.RawQuery, qt.Equals, tc.url.RawQuery)
			c.Assert(unmarshaledURL.Fragment, qt.Equals, tc.url.Fragment)
		})
	}
}

func TestURL_JSONUnmarshaling_Error(t *testing.T) {
	testCases := []struct {
		name      string
		jsonInput string
	}{
		{
			name:      "invalid JSON",
			jsonInput: `"invalid JSON`,
		},
		{
			name:      "invalid URL format",
			jsonInput: `"://invalid-url"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			var url models.URL
			err := json.Unmarshal([]byte(tc.jsonInput), &url)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

func TestURL_JSONUnmarshaling_NonString(t *testing.T) {
	c := qt.New(t)

	var url models.URL
	err := json.Unmarshal([]byte(`123`), &url)
	c.Assert(err, qt.IsNotNil)
}
