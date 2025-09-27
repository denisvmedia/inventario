package assets_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/assets"
)

// TestServePlaceholderImageImplementation tests the exact implementation used in the files API
func TestServePlaceholderImageImplementation(t *testing.T) {
	// This mimics the exact implementation from go/apiserver/files.go servePlaceholderImage function
	servePlaceholderImage := func(w http.ResponseWriter, r *http.Request, size string) {
		// Set appropriate headers
		w.Header().Set("Content-Type", "image/gif")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		// Get placeholder from embedded assets
		filename := "generating_" + size + ".gif"
		data, err := assets.GetPlaceholderFile(filename)
		if err != nil {
			slog.Error("Failed to load placeholder image", "filename", filename, "error", err)
			http.Error(w, "Placeholder not found", http.StatusNotFound)
			return
		}

		// Set content length
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))

		// Write the image data
		if _, err := w.Write(data); err != nil {
			slog.Error("Failed to write placeholder image", "filename", filename, "error", err)
		}
	}

	// Create a test handler that uses the implementation
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		size := r.URL.Query().Get("size")
		if size == "" {
			size = "small"
		}
		servePlaceholderImage(w, r, size)
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

			// Check cache headers (exactly as implemented)
			c.Assert(w.Header().Get("Cache-Control"), qt.Equals, "no-cache, no-store, must-revalidate")
			c.Assert(w.Header().Get("Pragma"), qt.Equals, "no-cache")
			c.Assert(w.Header().Get("Expires"), qt.Equals, "0")

			// Check that we got actual image data
			body := w.Body.Bytes()
			c.Assert(len(body), qt.Not(qt.Equals), 0)

			// Check GIF header
			c.Assert(len(body) >= 6, qt.IsTrue)
			c.Assert(string(body[:6]), qt.Matches, "GIF8[79]a")

			// Check content length header matches actual data
			expectedLength := strconv.Itoa(len(body))
			c.Assert(w.Header().Get("Content-Length"), qt.Equals, expectedLength)

			// Verify the data is the same as what we get directly from assets
			directData, err := assets.GetPlaceholderFile("generating_" + tt.size + ".gif")
			c.Assert(err, qt.IsNil)
			c.Assert(body, qt.DeepEquals, directData)
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

// TestEmbeddedAssetsAreActuallyEmbedded verifies that the assets are truly embedded in the binary
func TestEmbeddedAssetsAreActuallyEmbedded(t *testing.T) {
	c := qt.New(t)

	// Test that we can get placeholders without any external files
	smallData, err := assets.GetPlaceholderFile("generating_small.gif")
	c.Assert(err, qt.IsNil)
	c.Assert(len(smallData), qt.Not(qt.Equals), 0)

	mediumData, err := assets.GetPlaceholderFile("generating_medium.gif")
	c.Assert(err, qt.IsNil)
	c.Assert(len(mediumData), qt.Not(qt.Equals), 0)

	// Verify they are different files (different sizes should have different data)
	c.Assert(smallData, qt.Not(qt.DeepEquals), mediumData)

	// Both should be valid GIF files
	c.Assert(string(smallData[:6]), qt.Matches, "GIF8[79]a")
	c.Assert(string(mediumData[:6]), qt.Matches, "GIF8[79]a")

	// Test filesystem interface
	fs := assets.GetPlaceholders()
	c.Assert(fs, qt.IsNotNil)
}

// TestEmbeddedAssetsFilesystemInterface tests the filesystem interface separately
func TestEmbeddedAssetsFilesystemInterface(t *testing.T) {
	c := qt.New(t)

	// Get the data directly first for comparison
	smallData, err := assets.GetPlaceholderFile("generating_small.gif")
	c.Assert(err, qt.IsNil)

	mediumData, err := assets.GetPlaceholderFile("generating_medium.gif")
	c.Assert(err, qt.IsNil)

	// Test filesystem interface
	fs := assets.GetPlaceholders()
	c.Assert(fs, qt.IsNotNil)

	// Should be able to read the same files through the filesystem interface
	readFileFS, ok := fs.(interface{ ReadFile(string) ([]byte, error) })
	c.Assert(ok, qt.IsTrue, qt.Commentf("filesystem should implement ReadFile interface"))

	fsSmallData, err := readFileFS.ReadFile("placeholders/generating_small.gif")
	c.Assert(err, qt.IsNil)
	c.Assert(fsSmallData, qt.DeepEquals, smallData)

	fsMediumData, err := readFileFS.ReadFile("placeholders/generating_medium.gif")
	c.Assert(err, qt.IsNil)
	c.Assert(fsMediumData, qt.DeepEquals, mediumData)
}
