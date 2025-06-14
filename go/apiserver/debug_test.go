package apiserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/debug"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestDebugAPI(t *testing.T) {
	c := qt.New(t)

	// Create test registry set
	registrySet, err := memory.NewRegistrySet("")
	c.Assert(err, qt.IsNil)

	// Test cases for different configurations
	testCases := []struct {
		name               string
		uploadLocation     string
		debugInfo          *debug.Info
		expectedFileDriver string
		expectedDBDriver   string
	}{
		{
			name:               "memory database with file storage",
			uploadLocation:     "file:///tmp/uploads?create_dir=1",
			debugInfo:          debug.NewInfo("memory://", "file:///tmp/uploads?create_dir=1"),
			expectedFileDriver: "file",
			expectedDBDriver:   "memory",
		},
		{
			name:               "postgres database with s3 storage",
			uploadLocation:     "s3://my-bucket/uploads?region=us-east-1",
			debugInfo:          debug.NewInfo("postgres://user:pass@localhost:5432/db", "s3://my-bucket/uploads?region=us-east-1"),
			expectedFileDriver: "s3",
			expectedDBDriver:   "postgres",
		},
		{
			name:               "boltdb database with azure storage",
			uploadLocation:     "azblob://container/uploads",
			debugInfo:          debug.NewInfo("boltdb:///path/to/db.bolt", "azblob://container/uploads"),
			expectedFileDriver: "azblob",
			expectedDBDriver:   "boltdb",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			// Create API server with test parameters
			params := apiserver.Params{
				RegistrySet:    registrySet,
				UploadLocation: tc.uploadLocation,
				DebugInfo:      tc.debugInfo,
			}

			// Mock workers for testing
			mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
			mockImportWorker := &mockImportWorker{isRunning: false}
			server := apiserver.APIServer(params, mockRestoreWorker, mockImportWorker)

			// Create test request
			req := httptest.NewRequest(http.MethodGet, "/api/v1/debug", nil)
			req = req.WithContext(context.Background())
			w := httptest.NewRecorder()

			// Execute request
			server.ServeHTTP(w, req)

			// Check response status
			c.Assert(w.Code, qt.Equals, http.StatusOK)

			// Check content type
			c.Assert(w.Header().Get("Content-Type"), qt.Equals, "application/json")

			// Parse response body
			var debugInfo debug.Info
			err := json.NewDecoder(w.Body).Decode(&debugInfo)
			c.Assert(err, qt.IsNil)

			// Verify debug information
			c.Assert(debugInfo.FileStorageDriver, qt.Equals, tc.expectedFileDriver)
			c.Assert(debugInfo.DatabaseDriver, qt.Equals, tc.expectedDBDriver)
			c.Assert(debugInfo.OperatingSystem, qt.Equals, runtime.GOOS)
		})
	}
}

func TestDebugAPI_InvalidURLs(t *testing.T) {
	c := qt.New(t)

	// Create test registry set
	registrySet, err := memory.NewRegistrySet("")
	c.Assert(err, qt.IsNil)

	// Test with invalid URLs
	params := apiserver.Params{
		RegistrySet:    registrySet,
		UploadLocation: "://invalid-url",
		DebugInfo:      debug.NewInfo("://invalid-dsn", "://invalid-url"),
	}

	// Mock workers for testing
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	mockImportWorker := &mockImportWorker{isRunning: false}
	server := apiserver.APIServer(params, mockRestoreWorker, mockImportWorker)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/debug", nil)
	req = req.WithContext(context.Background())
	w := httptest.NewRecorder()

	// Execute request
	server.ServeHTTP(w, req)

	// Check response status
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	// Parse response body
	var debugInfo debug.InfoJSON
	err = json.NewDecoder(w.Body).Decode(&debugInfo)
	c.Assert(err, qt.IsNil)

	// Should return "unknown" for invalid URLs
	c.Assert(debugInfo.FileStorageDriver, qt.Equals, "")
	c.Assert(debugInfo.DatabaseDriver, qt.Equals, "")
	c.Assert(debugInfo.OperatingSystem, qt.Equals, runtime.GOOS)
	c.Assert(debugInfo.Error, qt.IsNotNil)
}
