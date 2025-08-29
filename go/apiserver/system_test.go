package apiserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/debug"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestSystemAPI_GetSystemInfo(t *testing.T) {
	c := qt.New(t)

	// Create test registry set
	registrySet := memory.NewRegistrySet()
	c.Assert(registrySet, qt.IsNotNil)

	_, err := registrySet.UserRegistry.Create(c.Context(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
		Email:    "test@example.com",
		Name:     "Test User",
		Role:     models.UserRoleUser,
		IsActive: true,
	})
	c.Assert(err, qt.IsNil)
	_, err = registrySet.TenantRegistry.Create(c.Context(), models.Tenant{
		EntityID: models.EntityID{ID: "test-tenant-id"},
		Name:     "Test Tenant",
	})
	c.Assert(err, qt.IsNil)

	// Create test parameters
	startTime := time.Now().Add(-1 * time.Hour) // 1 hour ago
	params := apiserver.Params{
		RegistrySet:    registrySet,
		UploadLocation: "file:///tmp/uploads?create_dir=1",
		DebugInfo:      debug.NewInfo("memory://", "file:///tmp/uploads?create_dir=1"),
		StartTime:      startTime,
		JWTSecret:      testJWTSecret,
	}

	// Create API server
	server := apiserver.APIServer(params, &mockRestoreWorker{})

	// Create test request
	req := httptest.NewRequest("GET", "/api/v1/system", nil)
	req.Header.Set("Accept", "application/json")
	addTestUserAuthHeader(req)

	// Create response recorder
	w := httptest.NewRecorder()

	// Execute request
	server.ServeHTTP(w, req)

	// Check response
	c.Assert(w.Code, qt.Equals, http.StatusOK)
	c.Assert(w.Header().Get("Content-Type"), qt.Equals, "application/json")

	// Parse response
	var systemInfo apiserver.SystemInfo
	err = json.Unmarshal(w.Body.Bytes(), &systemInfo)
	c.Assert(err, qt.IsNil)

	// Verify system information
	c.Assert(systemInfo.Version, qt.Not(qt.Equals), "")
	c.Assert(systemInfo.GoVersion, qt.Equals, runtime.Version())
	c.Assert(systemInfo.DatabaseBackend, qt.Equals, "memory")
	c.Assert(systemInfo.FileStorageBackend, qt.Equals, "file")
	c.Assert(systemInfo.OperatingSystem, qt.Equals, runtime.GOOS)
	c.Assert(systemInfo.NumCPU, qt.Equals, runtime.NumCPU())
	c.Assert(systemInfo.NumGoroutines, qt.Not(qt.Equals), 0)
	c.Assert(systemInfo.Uptime, qt.Not(qt.Equals), "")
	c.Assert(systemInfo.MemoryUsage, qt.Not(qt.Equals), "")
}

func TestSystemAPI_GetSystemInfoWithSettings(t *testing.T) {
	c := qt.New(t)

	// Create test registry set
	registrySet := memory.NewRegistrySet()
	c.Assert(registrySet, qt.IsNotNil)

	// Create test user
	testUser := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
		Email:    "test@example.com",
		Name:     "Test User",
		Role:     models.UserRoleUser,
		IsActive: true,
	}
	err := testUser.SetPassword("testpassword123")
	c.Assert(err, qt.IsNil)
	_, err = registrySet.UserRegistry.Create(c.Context(), testUser)
	c.Assert(err, qt.IsNil)
	_, err = registrySet.TenantRegistry.Create(c.Context(), models.Tenant{
		EntityID: models.EntityID{ID: "test-tenant-id"},
		Name:     "Test Tenant",
	})
	c.Assert(err, qt.IsNil)

	// Add some test settings
	testSettings := models.SettingsObject{
		MainCurrency: ptr("USD"),
		Theme:        ptr("dark"),
	}

	userCtx := appctx.WithUser(c.Context(), &testUser)
	settingsRegistry, err := registrySet.SettingsRegistry.WithCurrentUser(userCtx)
	c.Assert(err, qt.IsNil)
	err = settingsRegistry.Save(c.Context(), testSettings)
	c.Assert(err, qt.IsNil)

	// Create test parameters
	startTime := time.Now().Add(-30 * time.Minute) // 30 minutes ago
	params := apiserver.Params{
		RegistrySet:    registrySet,
		UploadLocation: "s3://my-bucket/uploads?region=us-east-1",
		DebugInfo:      debug.NewInfo("postgres://user:pass@localhost:5432/db", "s3://my-bucket/uploads?region=us-east-1"),
		StartTime:      startTime,
		JWTSecret:      testJWTSecret,
	}

	// Create API server
	server := apiserver.APIServer(params, &mockRestoreWorker{})

	// Create test request
	req := httptest.NewRequest("GET", "/api/v1/system", nil)
	req.Header.Set("Accept", "application/json")
	addTestUserAuthHeader(req)

	// Create response recorder
	w := httptest.NewRecorder()

	// Execute request
	server.ServeHTTP(w, req)

	// Check response
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	// Parse response
	var systemInfo apiserver.SystemInfo
	err = json.Unmarshal(w.Body.Bytes(), &systemInfo)
	c.Assert(err, qt.IsNil)

	// Verify system information
	c.Assert(systemInfo.DatabaseBackend, qt.Equals, "postgres")
	c.Assert(systemInfo.FileStorageBackend, qt.Equals, "s3")

	// Verify settings are included
	c.Assert(systemInfo.Settings.MainCurrency, qt.IsNotNil)
	c.Assert(*systemInfo.Settings.MainCurrency, qt.Equals, "USD")
	c.Assert(systemInfo.Settings.Theme, qt.IsNotNil)
	c.Assert(*systemInfo.Settings.Theme, qt.Equals, "dark")
}

// Helper function for pointer creation
func ptr[T any](v T) *T {
	return &v
}
