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

// GET /debug moved to /admin/debug behind the back-office auth plane
// (issue #2113, L-4). The endpoint leaks operational config (storage/db
// driver, OS) and must not be readable by an ordinary tenant user.

func TestDebugAPI(t *testing.T) {
	c := qt.New(t)

	// Create test factory set (not used directly, but needed for compilation)
	factorySet := memory.NewFactorySet()
	c.Assert(factorySet, qt.IsNotNil)

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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			// Create API server with test parameters
			params, _, _ := newParams()
			params.UploadLocation = tc.uploadLocation
			params.DebugInfo = tc.debugInfo

			// A back-office admin is required to read /admin/debug.
			_, token := WithBackofficeAdmin(t, params)

			mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
			server := apiserver.APIServer(params, mockRestoreWorker)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/debug", nil)
			req = req.WithContext(context.Background())
			addBackofficeAuthHeader(req, token)
			w := httptest.NewRecorder()

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

	// Create test factory set (not used directly, but needed for compilation)
	factorySet := memory.NewFactorySet()
	c.Assert(factorySet, qt.IsNotNil)

	// Test with invalid URLs
	params, _, _ := newParams()
	params.UploadLocation = "://invalid-url"
	params.DebugInfo = debug.NewInfo("://invalid-dsn", "://invalid-url")

	_, token := WithBackofficeAdmin(t, params)

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	server := apiserver.APIServer(params, mockRestoreWorker)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/debug", nil)
	req = req.WithContext(context.Background())
	addBackofficeAuthHeader(req, token)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	// Check response status
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	// Parse response body
	var debugInfo debug.InfoJSON
	err := json.NewDecoder(w.Body).Decode(&debugInfo)
	c.Assert(err, qt.IsNil)

	// Should return "unknown" for invalid URLs
	c.Assert(debugInfo.FileStorageDriver, qt.Equals, "")
	c.Assert(debugInfo.DatabaseDriver, qt.Equals, "")
	c.Assert(debugInfo.OperatingSystem, qt.Equals, runtime.GOOS)
	c.Assert(debugInfo.Error, qt.IsNotNil)
}

// TestDebugAPI_DeniesTenantUser asserts a plain tenant JWT cannot reach the
// back-office-gated debug endpoint (issue #2113, L-4).
func TestDebugAPI_DeniesTenantUser(t *testing.T) {
	c := qt.New(t)

	params, user, _ := newParams()
	params.DebugInfo = debug.NewInfo("memory://", uploadLocation)
	server := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/debug", nil)
	// Tenant JWT — RequireBackofficeAuth rejects it (audience mismatch /
	// missing admin_id).
	addTestUserAuthHeader(req, user.ID)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusUnauthorized)
}

// TestDebugAPI_DeniesUnauthenticated asserts an anonymous caller cannot reach
// the debug endpoint.
func TestDebugAPI_DeniesUnauthenticated(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams()
	params.DebugInfo = debug.NewInfo("memory://", uploadLocation)
	server := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/debug", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusUnauthorized)
}
