package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/backup/restore"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

func newTestFactorySet() (*registry.FactorySet, *models.User) {
	// Create factory set
	factorySet := memory.NewFactorySet()

	// Create a test user for authentication
	testUser := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			// ID will be generated server-side for security
			TenantID: "test-tenant-id",
		},
		Email:    "test@example.com",
		Name:     "Test User",
		Role:     models.UserRoleUser,
		IsActive: true,
	}
	testUser.SetPassword("password123")
	createdUser, _ := factorySet.UserRegistry.Create(context.Background(), testUser)

	return factorySet, createdUser
}

func TestRestoreConcurrencyControl_NoRunningRestore(t *testing.T) {
	c := qt.New(t)

	factorySet, testUser := newTestFactorySet()

	// Create an export first
	export := models.Export{
		Description: "Test Export",
		Status:      models.ExportStatusCompleted,
		FilePath:    "test-export.xml",
		CreatedDate: models.PNow(),
	}
	ctx := appctx.WithUser(context.Background(), testUser)
	exportRegistry := factorySet.ExportRegistryFactory.MustCreateUserRegistry(ctx)
	createdExport, err := exportRegistry.Create(context.Background(), export)
	c.Assert(err, qt.IsNil)

	// Set up router with authentication
	r := chi.NewRouter()
	r.Use(apiserver.JWTMiddleware(testJWTSecret, factorySet.UserRegistry))
	r.Use(apiserver.RegistrySetMiddleware(factorySet))

	params := apiserver.Params{
		FactorySet:     factorySet,
		UploadLocation: "memory://",
		JWTSecret:      testJWTSecret,
	}

	// Mock worker with no running restores
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	r.Route("/api/v1/exports", apiserver.Exports(params, mockRestoreWorker))

	// Create restore request
	restoreRequest := &jsonapi.RestoreOperationCreateRequest{
		Data: &jsonapi.RestoreOperationCreateRequestData{
			Type: "restores",
			Attributes: &models.RestoreOperation{
				Description: "Test Restore",
				Options: models.RestoreOptions{
					Strategy:        "merge_update",
					IncludeFileData: false,
					DryRun:          false,
				},
			},
		},
	}

	data, err := json.Marshal(restoreRequest)
	c.Assert(err, qt.IsNil)

	req, err := http.NewRequest("POST", "/api/v1/exports/"+createdExport.ID+"/restores", bytes.NewReader(data))
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", "application/json")
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// Should succeed when no restore is running
	c.Assert(rr.Code, qt.Equals, http.StatusCreated)
}

func TestRestoreConcurrencyControl_RestoreAlreadyRunning(t *testing.T) {
	c := qt.New(t)

	factorySet, testUser := newTestFactorySet()

	// Create an export first
	export := models.Export{
		Description: "Test Export",
		Status:      models.ExportStatusCompleted,
		FilePath:    "test-export.xml",
		CreatedDate: models.PNow(),
	}
	ctx := appctx.WithUser(context.Background(), testUser)
	exportRegistry := factorySet.ExportRegistryFactory.MustCreateUserRegistry(ctx)
	createdExport, err := exportRegistry.Create(context.Background(), export)
	c.Assert(err, qt.IsNil)

	// Set up router with authentication
	r := chi.NewRouter()
	r.Use(apiserver.JWTMiddleware(testJWTSecret, factorySet.UserRegistry))
	r.Use(apiserver.RegistrySetMiddleware(factorySet))

	params := apiserver.Params{
		FactorySet:     factorySet,
		UploadLocation: "memory://",
		JWTSecret:      testJWTSecret,
	}

	// Mock worker with running restore
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: true}
	r.Route("/api/v1/exports", apiserver.Exports(params, mockRestoreWorker))

	// Create restore request
	restoreRequest := &jsonapi.RestoreOperationCreateRequest{
		Data: &jsonapi.RestoreOperationCreateRequestData{
			Type: "restores",
			Attributes: &models.RestoreOperation{
				Description: "Test Restore",
				Options: models.RestoreOptions{
					Strategy:        "merge_update",
					IncludeFileData: false,
					DryRun:          false,
				},
			},
		},
	}

	data, err := json.Marshal(restoreRequest)
	c.Assert(err, qt.IsNil)

	req, err := http.NewRequest("POST", "/api/v1/exports/"+createdExport.ID+"/restores", bytes.NewReader(data))
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", "application/json")
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// Should return 409 Conflict when restore is already running
	c.Assert(rr.Code, qt.Equals, http.StatusConflict)

	// Check error message
	var response map[string]any
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	c.Assert(err, qt.IsNil)

	errors, ok := response["errors"].([]any)
	c.Assert(ok, qt.IsTrue)
	c.Assert(errors, qt.HasLen, 1)

	errorObj, ok := errors[0].(map[string]any)
	c.Assert(ok, qt.IsTrue)
	c.Assert(errorObj["status"], qt.Equals, "Conflict")

	// Verify the error message contains the expected text and is user-friendly
	errorDetails, ok := errorObj["error"].(map[string]any)
	c.Assert(ok, qt.IsTrue)
	errorMsg, ok := errorDetails["msg"].(string)
	c.Assert(ok, qt.IsTrue)
	c.Assert(errorMsg, qt.Matches, ".*restore operation is already in progress.*")

	// Verify it's a user-friendly message (not a technical error)
	c.Assert(errorMsg, qt.Matches, ".*Please wait for it to complete.*")
}

func TestRestoreConcurrencyControl_PendingRestoreBlocks(t *testing.T) {
	c := qt.New(t)

	factorySet, testUser := newTestFactorySet()

	// Create an export first
	export := models.Export{
		Description: "Test Export",
		Status:      models.ExportStatusCompleted,
		FilePath:    "test-export.xml",
		CreatedDate: models.PNow(),
	}
	ctx := appctx.WithUser(context.Background(), testUser)
	exportRegistry := factorySet.ExportRegistryFactory.MustCreateUserRegistry(ctx)
	createdExport, err := exportRegistry.Create(context.Background(), export)
	c.Assert(err, qt.IsNil)

	// Create a pending restore operation directly in the database
	pendingRestore := models.RestoreOperation{
		ExportID:    createdExport.ID,
		Description: "Pending Restore",
		Status:      models.RestoreStatusPending,
		Options: models.RestoreOptions{
			Strategy:        "merge_update",
			IncludeFileData: false,
			DryRun:          false,
		},
		CreatedDate: models.PNow(),
	}
	restoreOpRegistry := factorySet.RestoreOperationRegistryFactory.MustCreateUserRegistry(ctx)
	_, err = restoreOpRegistry.Create(context.Background(), pendingRestore)
	c.Assert(err, qt.IsNil)

	// Set up router with authentication
	r := chi.NewRouter()
	r.Use(apiserver.JWTMiddleware(testJWTSecret, factorySet.UserRegistry))
	r.Use(apiserver.RegistrySetMiddleware(factorySet))

	params := apiserver.Params{
		FactorySet:     factorySet,
		UploadLocation: "memory://",
		JWTSecret:      testJWTSecret,
	}

	// Use real restore worker (not mock) to test actual logic
	entityService := services.NewEntityService(factorySet, "memory://")
	restoreService := restore.NewRestoreService(factorySet, entityService, "memory://")
	// Create user registry set for RestoreWorker
	userRegistrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))
	restoreWorker := restore.NewRestoreWorker(restoreService, userRegistrySet, "memory://")
	r.Route("/api/v1/exports", apiserver.Exports(params, restoreWorker))

	// Try to create another restore request
	restoreRequest := &jsonapi.RestoreOperationCreateRequest{
		Data: &jsonapi.RestoreOperationCreateRequestData{
			Type: "restores",
			Attributes: &models.RestoreOperation{
				Description: "Second Restore",
				Options: models.RestoreOptions{
					Strategy:        "merge_update",
					IncludeFileData: false,
					DryRun:          false,
				},
			},
		},
	}

	data, err := json.Marshal(restoreRequest)
	c.Assert(err, qt.IsNil)

	req, err := http.NewRequest("POST", "/api/v1/exports/"+createdExport.ID+"/restores", bytes.NewReader(data))
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", "application/json")
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// Should return 409 Conflict when a pending restore exists
	c.Assert(rr.Code, qt.Equals, http.StatusConflict)

	// Check error message
	var response map[string]any
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	c.Assert(err, qt.IsNil)

	errors, ok := response["errors"].([]any)
	c.Assert(ok, qt.IsTrue)
	c.Assert(errors, qt.HasLen, 1)

	errorObj, ok := errors[0].(map[string]any)
	c.Assert(ok, qt.IsTrue)
	c.Assert(errorObj["status"], qt.Equals, "Conflict")

	// Verify the error message mentions pending operations and is user-friendly
	errorDetails, ok := errorObj["error"].(map[string]any)
	c.Assert(ok, qt.IsTrue)
	errorMsg, ok := errorDetails["msg"].(string)
	c.Assert(ok, qt.IsTrue)
	c.Assert(errorMsg, qt.Matches, ".*restore operation is already in progress or pending.*")

	// Verify it's a user-friendly message (not a technical error)
	c.Assert(errorMsg, qt.Matches, ".*Please wait for it to complete.*")
}

func TestRestoreOperationCreatedWithPendingStatus(t *testing.T) {
	c := qt.New(t)

	factorySet, testUser := newTestFactorySet()

	// Create an export first
	export := models.Export{
		Description: "Test Export",
		Status:      models.ExportStatusCompleted,
		FilePath:    "test-export.xml",
		CreatedDate: models.PNow(),
	}
	ctx := appctx.WithUser(context.Background(), testUser)
	exportRegistry := factorySet.ExportRegistryFactory.MustCreateUserRegistry(ctx)
	createdExport, err := exportRegistry.Create(context.Background(), export)
	c.Assert(err, qt.IsNil)

	// Set up router with authentication
	r := chi.NewRouter()
	r.Use(apiserver.JWTMiddleware(testJWTSecret, factorySet.UserRegistry))
	r.Use(apiserver.RegistrySetMiddleware(factorySet))

	params := apiserver.Params{
		FactorySet:     factorySet,
		UploadLocation: "memory://",
		JWTSecret:      testJWTSecret,
	}

	// Mock worker with no running restores
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	r.Route("/api/v1/exports", apiserver.Exports(params, mockRestoreWorker))

	// Create restore request
	restoreRequest := &jsonapi.RestoreOperationCreateRequest{
		Data: &jsonapi.RestoreOperationCreateRequestData{
			Type: "restores",
			Attributes: &models.RestoreOperation{
				Description: "Test Restore",
				Options: models.RestoreOptions{
					Strategy:        "merge_update",
					IncludeFileData: false,
					DryRun:          false,
				},
			},
		},
	}

	data, err := json.Marshal(restoreRequest)
	c.Assert(err, qt.IsNil)

	req, err := http.NewRequest("POST", "/api/v1/exports/"+createdExport.ID+"/restores", bytes.NewReader(data))
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", "application/json")
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusCreated)

	// Parse response to get the restore operation ID
	var response map[string]any
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	c.Assert(err, qt.IsNil)

	dataObj, ok := response["data"].(map[string]any)
	c.Assert(ok, qt.IsTrue)

	restoreID, ok := dataObj["id"].(string)
	c.Assert(ok, qt.IsTrue)

	// Verify the restore operation was created with pending status
	restoreOpRegistry := factorySet.RestoreOperationRegistryFactory.MustCreateUserRegistry(ctx)
	restoreOp, err := restoreOpRegistry.Get(context.Background(), restoreID)
	c.Assert(err, qt.IsNil)
	c.Assert(restoreOp.Status, qt.Equals, models.RestoreStatusPending)
}
