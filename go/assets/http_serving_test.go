package assets_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/assets"
)

func TestPlaceholderHTTPServing(t *testing.T) {
	// Create a simple HTTP handler that serves placeholders like the real implementation
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract size from URL path
		size := "small" // Default for this test
		if r.URL.Query().Get("size") != "" {
			size = r.URL.Query().Get("size")
		}

		// Set headers like the real implementation
		w.Header().Set("Content-Type", "image/gif")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		// Get placeholder from embedded assets
		filename := "generating_" + size + ".gif"
		data, err := assets.GetPlaceholderFile(filename)
		if err != nil {
			http.Error(w, "Placeholder not found", http.StatusNotFound)
			return
		}

		// Set content length and write data
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		if _, err := w.Write(data); err != nil {
			http.Error(w, "Failed to write data", http.StatusInternalServerError)
		}
	})

	successTests := []struct {
		name string
		size string
	}{
		{
			name: "small placeholder",
			size: "small",
		},
		{
			name: "medium placeholder",
			size: "medium",
		},
	}

	for _, tt := range successTests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			req := httptest.NewRequest("GET", "/?size="+tt.size, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			c.Assert(w.Code, qt.Equals, http.StatusOK)

			// Check content type
			c.Assert(w.Header().Get("Content-Type"), qt.Equals, "image/gif")

			// Check cache headers
			c.Assert(w.Header().Get("Cache-Control"), qt.Equals, "no-cache, no-store, must-revalidate")
			c.Assert(w.Header().Get("Pragma"), qt.Equals, "no-cache")
			c.Assert(w.Header().Get("Expires"), qt.Equals, "0")

			// Check that we got actual image data
			body := w.Body.Bytes()
			c.Assert(len(body), qt.Not(qt.Equals), 0)

			// Check GIF header
			c.Assert(len(body) >= 6, qt.IsTrue)
			c.Assert(string(body[:6]), qt.Matches, "GIF8[79]a")

			// Check content length header
			c.Assert(w.Header().Get("Content-Length"), qt.Not(qt.Equals), "")

			// Verify content length matches actual data
			expectedLength := fmt.Sprintf("%d", len(body))
			c.Assert(w.Header().Get("Content-Length"), qt.Equals, expectedLength)
		})
	}

	errorTests := []struct {
		name string
		size string
	}{
		{
			name: "invalid size",
			size: "invalid",
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			req := httptest.NewRequest("GET", "/?size="+tt.size, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			c.Assert(w.Code, qt.Equals, http.StatusNotFound)
		})
	}
}
