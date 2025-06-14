package apiserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// mockRestoreWorker is a mock implementation of RestoreWorkerInterface for testing
type mockRestoreWorker struct {
	hasRunningRestores bool
}

func (m *mockRestoreWorker) HasRunningRestores(ctx context.Context) (bool, error) {
	return m.hasRunningRestores, nil
}

func TestExportSoftDelete(t *testing.T) {
	c := qt.New(t)

	// Create test registry
	registrySet := &registry.Set{
		ExportRegistry: memory.NewExportRegistry(),
	}

	// Create test export
	export := models.Export{
		Type:        models.ExportTypeFullDatabase,
		Description: "Test export for soft delete",
		Status:      models.ExportStatusCompleted,
		FilePath:    "test/export.xml",
	}

	created, err := registrySet.ExportRegistry.Create(context.Background(), export)
	c.Assert(err, qt.IsNil)

	// Create router with export routes
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	params := Params{
		RegistrySet:    registrySet,
		UploadLocation: "memory://",
	}
	mockWorker := &mockRestoreWorker{hasRunningRestores: false}
	r.Route("/exports", Exports(params, mockWorker))

	// Test soft delete
	req := httptest.NewRequest("DELETE", "/exports/"+created.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusNoContent)

	// Verify export is soft deleted
	retrieved, err := registrySet.ExportRegistry.Get(context.Background(), created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved.IsDeleted(), qt.IsTrue)

	// Test that download is blocked for deleted export
	req = httptest.NewRequest("GET", "/exports/"+created.ID+"/download", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusNotFound)
}

func TestExportListExcludesDeleted(t *testing.T) {
	c := qt.New(t)

	// Create test registry
	registrySet := &registry.Set{
		ExportRegistry: memory.NewExportRegistry(),
	}

	// Create test exports
	export1 := models.Export{
		Type:        models.ExportTypeFullDatabase,
		Description: "Active export",
		Status:      models.ExportStatusCompleted,
	}

	export2 := models.Export{
		Type:        models.ExportTypeLocations,
		Description: "Export to be deleted",
		Status:      models.ExportStatusCompleted,
	}

	created1, err := registrySet.ExportRegistry.Create(context.Background(), export1)
	c.Assert(err, qt.IsNil)

	created2, err := registrySet.ExportRegistry.Create(context.Background(), export2)
	c.Assert(err, qt.IsNil)

	// Soft delete one export
	err = registrySet.ExportRegistry.Delete(context.Background(), created2.ID)
	c.Assert(err, qt.IsNil)

	// Create router with export routes
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	params := Params{
		RegistrySet:    registrySet,
		UploadLocation: "memory://",
	}
	mockWorker := &mockRestoreWorker{hasRunningRestores: false}
	r.Route("/exports", Exports(params, mockWorker))

	// Test list endpoint
	req := httptest.NewRequest("GET", "/exports", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusOK)

	var response jsonapi.ExportsResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	c.Assert(err, qt.IsNil)

	// Should only return the active export
	c.Assert(response.Data, qt.HasLen, 1)
	c.Assert(response.Data[0].ID, qt.Equals, created1.ID)
}

func TestExportListWithDeletedParameter(t *testing.T) {
	c := qt.New(t)

	// Create test registry
	registrySet := &registry.Set{
		ExportRegistry: memory.NewExportRegistry(),
	}

	// Create test exports
	export1 := models.Export{
		Type:        models.ExportTypeFullDatabase,
		Description: "Active export",
		Status:      models.ExportStatusCompleted,
	}

	export2 := models.Export{
		Type:        models.ExportTypeLocations,
		Description: "Export to be deleted",
		Status:      models.ExportStatusCompleted,
	}

	created1, err := registrySet.ExportRegistry.Create(context.Background(), export1)
	c.Assert(err, qt.IsNil)

	created2, err := registrySet.ExportRegistry.Create(context.Background(), export2)
	c.Assert(err, qt.IsNil)

	// Soft delete one export
	err = registrySet.ExportRegistry.Delete(context.Background(), created2.ID)
	c.Assert(err, qt.IsNil)

	// Create router with export routes
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	params := Params{
		RegistrySet:    registrySet,
		UploadLocation: "memory://",
	}
	mockWorker := &mockRestoreWorker{hasRunningRestores: false}
	r.Route("/exports", Exports(params, mockWorker))

	// Test list endpoint with include_deleted=true
	req := httptest.NewRequest("GET", "/exports?include_deleted=true", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusOK)

	var response jsonapi.ExportsResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	c.Assert(err, qt.IsNil)

	// Should return both exports (active and deleted)
	c.Assert(response.Data, qt.HasLen, 2)

	// Verify we have both exports
	exportIDs := make([]string, len(response.Data))
	for i, exp := range response.Data {
		exportIDs[i] = exp.ID
	}
	c.Assert(exportIDs, qt.Contains, created1.ID)
	c.Assert(exportIDs, qt.Contains, created2.ID)
}

func TestExportCreate_SetsCreatedDate(t *testing.T) {
	c := qt.New(t)

	// Create test registry
	registrySet := &registry.Set{
		ExportRegistry: memory.NewExportRegistry(),
	}

	// Create router with export routes
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	params := Params{
		RegistrySet:    registrySet,
		UploadLocation: "memory://",
	}
	mockWorker := &mockRestoreWorker{hasRunningRestores: false}
	r.Route("/exports", Exports(params, mockWorker))

	// Create export request payload
	requestPayload := jsonapi.ExportCreateRequest{
		Data: &jsonapi.ExportCreateRequestData{
			Type: "exports",
			Attributes: &models.Export{
				Type:        models.ExportTypeFullDatabase,
				Description: "Test export for timestamp",
				// CreatedDate is not set - should be set by API
			},
		},
	}

	payloadBytes, err := json.Marshal(requestPayload)
	c.Assert(err, qt.IsNil)

	// Test create endpoint
	req := httptest.NewRequest("POST", "/exports", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusCreated)

	var response jsonapi.ExportResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	c.Assert(err, qt.IsNil)

	// Verify that created_date was set by the API
	c.Assert(response.Data.Attributes.CreatedDate, qt.IsNotNil)
	c.Assert(response.Data.Attributes.Status, qt.Equals, models.ExportStatusPending)
	c.Assert(response.Data.Attributes.Description, qt.Equals, "Test export for timestamp")

	// Verify the timestamp is in the correct RFC3339 format
	createdDateStr := string(*response.Data.Attributes.CreatedDate)
	c.Assert(strings.Contains(createdDateStr, "T"), qt.IsTrue, qt.Commentf("Expected RFC3339 format with 'T', got: %s", createdDateStr))
	// RFC3339 can end with 'Z' (UTC) or timezone offset like '+02:00'
	hasValidTimezone := strings.HasSuffix(createdDateStr, "Z") || strings.Contains(createdDateStr, "+") || strings.Contains(createdDateStr, "-")
	c.Assert(hasValidTimezone, qt.IsTrue, qt.Commentf("Expected RFC3339 format with timezone, got: %s", createdDateStr))
}
