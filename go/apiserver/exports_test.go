package apiserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestExportSoftDelete(t *testing.T) {
	c := quicktest.New(t)

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
	c.Assert(err, quicktest.IsNil)

	// Create router with export routes
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	params := Params{
		RegistrySet:    registrySet,
		UploadLocation: "memory://",
	}
	r.Route("/exports", Exports(params))

	// Test soft delete
	req := httptest.NewRequest("DELETE", "/exports/"+created.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, quicktest.Equals, http.StatusNoContent)

	// Verify export is soft deleted
	retrieved, err := registrySet.ExportRegistry.Get(context.Background(), created.ID)
	c.Assert(err, quicktest.IsNil)
	c.Assert(retrieved.IsDeleted(), quicktest.IsTrue)

	// Test that download is blocked for deleted export
	req = httptest.NewRequest("GET", "/exports/"+created.ID+"/download", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, quicktest.Equals, http.StatusNotFound)
}

func TestExportListExcludesDeleted(t *testing.T) {
	c := quicktest.New(t)

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
	c.Assert(err, quicktest.IsNil)

	created2, err := registrySet.ExportRegistry.Create(context.Background(), export2)
	c.Assert(err, quicktest.IsNil)

	// Soft delete one export
	err = registrySet.ExportRegistry.Delete(context.Background(), created2.ID)
	c.Assert(err, quicktest.IsNil)

	// Create router with export routes
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	params := Params{
		RegistrySet:    registrySet,
		UploadLocation: "memory://",
	}
	r.Route("/exports", Exports(params))

	// Test list endpoint
	req := httptest.NewRequest("GET", "/exports", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, quicktest.Equals, http.StatusOK)

	var response jsonapi.ExportsResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	c.Assert(err, quicktest.IsNil)

	// Should only return the active export
	c.Assert(response.Data, quicktest.HasLen, 1)
	c.Assert(response.Data[0].ID, quicktest.Equals, created1.ID)
}
