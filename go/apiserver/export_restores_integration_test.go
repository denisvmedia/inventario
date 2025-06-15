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

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/backup/restore"
)

func newTestRegistrySet() *registry.Set {
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)
	commodityRegistry := memory.NewCommodityRegistry(areaRegistry)
	restoreStepRegistry := memory.NewRestoreStepRegistry()

	return &registry.Set{
		LocationRegistry:         locationRegistry,
		AreaRegistry:             areaRegistry,
		CommodityRegistry:        commodityRegistry,
		ImageRegistry:            memory.NewImageRegistry(commodityRegistry),
		InvoiceRegistry:          memory.NewInvoiceRegistry(commodityRegistry),
		ManualRegistry:           memory.NewManualRegistry(commodityRegistry),
		SettingsRegistry:         memory.NewSettingsRegistry(),
		ExportRegistry:           memory.NewExportRegistry(),
		RestoreOperationRegistry: memory.NewRestoreOperationRegistry(restoreStepRegistry),
		RestoreStepRegistry:      restoreStepRegistry,
	}
}

func TestRestoreConcurrencyControl_NoRunningRestore(t *testing.T) {
	c := qt.New(t)

	registrySet := newTestRegistrySet()

	// Create an export first
	export := models.Export{
		Description: "Test Export",
		Status:      models.ExportStatusCompleted,
		FilePath:    "test-export.xml",
		CreatedDate: models.PNow(),
	}
	createdExport, err := registrySet.ExportRegistry.Create(context.Background(), export)
	c.Assert(err, qt.IsNil)

	// Set up router
	r := chi.NewRouter()
	params := apiserver.Params{
		RegistrySet:    registrySet,
		UploadLocation: "memory://",
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
					BackupExisting:  false,
				},
			},
		},
	}

	data, err := json.Marshal(restoreRequest)
	c.Assert(err, qt.IsNil)

	req, err := http.NewRequest("POST", "/api/v1/exports/"+createdExport.ID+"/restores", bytes.NewReader(data))
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// Should succeed when no restore is running
	c.Assert(rr.Code, qt.Equals, http.StatusCreated)
}

func TestRestoreConcurrencyControl_RestoreAlreadyRunning(t *testing.T) {
	c := qt.New(t)

	registrySet := newTestRegistrySet()

	// Create an export first
	export := models.Export{
		Description: "Test Export",
		Status:      models.ExportStatusCompleted,
		FilePath:    "test-export.xml",
		CreatedDate: models.PNow(),
	}
	createdExport, err := registrySet.ExportRegistry.Create(context.Background(), export)
	c.Assert(err, qt.IsNil)

	// Set up router
	r := chi.NewRouter()
	params := apiserver.Params{
		RegistrySet:    registrySet,
		UploadLocation: "memory://",
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
					BackupExisting:  false,
				},
			},
		},
	}

	data, err := json.Marshal(restoreRequest)
	c.Assert(err, qt.IsNil)

	req, err := http.NewRequest("POST", "/api/v1/exports/"+createdExport.ID+"/restores", bytes.NewReader(data))
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// Should return 409 Conflict when restore is already running
	c.Assert(rr.Code, qt.Equals, http.StatusConflict)

	// Check error message
	var response map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	c.Assert(err, qt.IsNil)

	errors, ok := response["errors"].([]interface{})
	c.Assert(ok, qt.IsTrue)
	c.Assert(len(errors), qt.Equals, 1)

	errorObj, ok := errors[0].(map[string]interface{})
	c.Assert(ok, qt.IsTrue)
	c.Assert(errorObj["status"], qt.Equals, "Conflict")

	// Verify the error message contains the expected text and is user-friendly
	errorDetails, ok := errorObj["error"].(map[string]interface{})
	c.Assert(ok, qt.IsTrue)
	errorMsg, ok := errorDetails["msg"].(string)
	c.Assert(ok, qt.IsTrue)
	c.Assert(errorMsg, qt.Matches, ".*restore operation is already in progress.*")

	// Verify it's a user-friendly message (not a technical error)
	c.Assert(errorMsg, qt.Matches, ".*Please wait for it to complete.*")
}

func TestRestoreConcurrencyControl_PendingRestoreBlocks(t *testing.T) {
	c := qt.New(t)

	registrySet := newTestRegistrySet()

	// Create an export first
	export := models.Export{
		Description: "Test Export",
		Status:      models.ExportStatusCompleted,
		FilePath:    "test-export.xml",
		CreatedDate: models.PNow(),
	}
	createdExport, err := registrySet.ExportRegistry.Create(context.Background(), export)
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
			BackupExisting:  false,
		},
		CreatedDate: models.PNow(),
	}
	_, err = registrySet.RestoreOperationRegistry.Create(context.Background(), pendingRestore)
	c.Assert(err, qt.IsNil)

	// Set up router
	r := chi.NewRouter()
	params := apiserver.Params{
		RegistrySet:    registrySet,
		UploadLocation: "memory://",
	}

	// Use real restore worker (not mock) to test actual logic
	restoreService := restore.NewRestoreService(registrySet, "memory://")
	restoreWorker := restore.NewRestoreWorker(restoreService, registrySet, "memory://")
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
					BackupExisting:  false,
				},
			},
		},
	}

	data, err := json.Marshal(restoreRequest)
	c.Assert(err, qt.IsNil)

	req, err := http.NewRequest("POST", "/api/v1/exports/"+createdExport.ID+"/restores", bytes.NewReader(data))
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// Should return 409 Conflict when a pending restore exists
	c.Assert(rr.Code, qt.Equals, http.StatusConflict)

	// Check error message
	var response map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	c.Assert(err, qt.IsNil)

	errors, ok := response["errors"].([]interface{})
	c.Assert(ok, qt.IsTrue)
	c.Assert(len(errors), qt.Equals, 1)

	errorObj, ok := errors[0].(map[string]interface{})
	c.Assert(ok, qt.IsTrue)
	c.Assert(errorObj["status"], qt.Equals, "Conflict")

	// Verify the error message mentions pending operations and is user-friendly
	errorDetails, ok := errorObj["error"].(map[string]interface{})
	c.Assert(ok, qt.IsTrue)
	errorMsg, ok := errorDetails["msg"].(string)
	c.Assert(ok, qt.IsTrue)
	c.Assert(errorMsg, qt.Matches, ".*restore operation is already in progress or pending.*")

	// Verify it's a user-friendly message (not a technical error)
	c.Assert(errorMsg, qt.Matches, ".*Please wait for it to complete.*")
}

func TestRestoreOperationCreatedWithPendingStatus(t *testing.T) {
	c := qt.New(t)

	registrySet := newTestRegistrySet()

	// Create an export first
	export := models.Export{
		Description: "Test Export",
		Status:      models.ExportStatusCompleted,
		FilePath:    "test-export.xml",
		CreatedDate: models.PNow(),
	}
	createdExport, err := registrySet.ExportRegistry.Create(context.Background(), export)
	c.Assert(err, qt.IsNil)

	// Set up router
	r := chi.NewRouter()
	params := apiserver.Params{
		RegistrySet:    registrySet,
		UploadLocation: "memory://",
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
					BackupExisting:  false,
				},
			},
		},
	}

	data, err := json.Marshal(restoreRequest)
	c.Assert(err, qt.IsNil)

	req, err := http.NewRequest("POST", "/api/v1/exports/"+createdExport.ID+"/restores", bytes.NewReader(data))
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusCreated)

	// Parse response to get the restore operation ID
	var response map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	c.Assert(err, qt.IsNil)

	data_obj, ok := response["data"].(map[string]interface{})
	c.Assert(ok, qt.IsTrue)

	restoreID, ok := data_obj["id"].(string)
	c.Assert(ok, qt.IsTrue)

	// Verify the restore operation was created with pending status
	restoreOp, err := registrySet.RestoreOperationRegistry.Get(context.Background(), restoreID)
	c.Assert(err, qt.IsNil)
	c.Assert(restoreOp.Status, qt.Equals, models.RestoreStatusPending)
}
