package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

func TestExportHardDelete(t *testing.T) {
	c := qt.New(t)

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
		IsActive: true,
	}
	testUser.SetPassword("Password123")
	createdUser := must.Must(factorySet.UserRegistry.Create(context.Background(), testUser))

	// Create user context and get user-aware registry set
	ctx := appctx.WithUser(context.Background(), createdUser)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	// Create test export
	export := models.Export{
		Type:        models.ExportTypeFullDatabase,
		Description: "Test export for soft delete",
		Status:      models.ExportStatusCompleted,
		FilePath:    "test/export.xml",
	}

	created, err := registrySet.ExportRegistry.Create(ctx, export)
	c.Assert(err, qt.IsNil)

	// Create router with export routes and authentication
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(apiserver.JWTMiddleware(testJWTSecret, factorySet.UserRegistry, nil))
	r.Use(apiserver.RegistrySetMiddleware(factorySet))

	params := apiserver.Params{
		FactorySet:     factorySet,
		UploadLocation: "memory://",
		EntityService:  services.NewEntityService(factorySet, "memory://"),
		JWTSecret:      testJWTSecret,
	}
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	r.Route("/exports", apiserver.Exports(params, mockRestoreWorker))

	// Test hard delete
	req := httptest.NewRequest("DELETE", "/exports/"+created.ID, nil)
	addTestUserAuthHeader(req, createdUser.ID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusNoContent)

	// Verify export is hard deleted (completely gone)
	_, err = registrySet.ExportRegistry.Get(context.Background(), created.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Test that download is blocked for deleted export
	req = httptest.NewRequest("GET", "/exports/"+created.ID+"/download", nil)
	addTestUserAuthHeader(req, createdUser.ID)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusNotFound)
}

func TestExportListExcludesDeleted(t *testing.T) {
	c := qt.New(t)

	// Create factory set and test user
	factorySet := memory.NewFactorySet()
	testUserTemplate := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "test-tenant-id"},
		Email:               "test@example.com", Name: "Test User", IsActive: true,
	}
	must.Assert(testUserTemplate.SetPassword("Password123"))
	testUser := must.Must(factorySet.UserRegistry.Create(context.Background(), testUserTemplate))

	// Create user context and get user-aware registry set
	ctx := appctx.WithUser(context.Background(), testUser)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	// Create test exports
	export1 := models.Export{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID:        models.EntityID{ID: "export1"},
			TenantID:        "test-tenant-id",
			CreatedByUserID: testUser.ID,
		},
		Type:        models.ExportTypeFullDatabase,
		Description: "Active export",
		Status:      models.ExportStatusCompleted,
	}

	export2 := models.Export{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID:        models.EntityID{ID: "export2"},
			TenantID:        "test-tenant-id",
			CreatedByUserID: testUser.ID,
		},
		Type:        models.ExportTypeLocations,
		Description: "Export to be deleted",
		Status:      models.ExportStatusCompleted,
	}

	created1, err := registrySet.ExportRegistry.Create(ctx, export1)
	c.Assert(err, qt.IsNil)

	created2, err := registrySet.ExportRegistry.Create(ctx, export2)
	c.Assert(err, qt.IsNil)

	// Soft delete one export
	err = registrySet.ExportRegistry.Delete(ctx, created2.ID)
	c.Assert(err, qt.IsNil)

	// Create router with export routes
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	params := apiserver.Params{
		FactorySet:     factorySet,
		UploadLocation: "memory://",
	}
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	r.With(apiserver.RequireAuth(testJWTSecret, factorySet.UserRegistry, nil)).With(apiserver.RegistrySetMiddleware(factorySet)).Route("/exports", apiserver.Exports(params, mockRestoreWorker))

	// Test list endpoint
	req := httptest.NewRequest("GET", "/exports", nil)
	addTestUserAuthHeader(req, testUser.ID)
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

	// Create factory set and test user
	factorySet := memory.NewFactorySet()
	testUserTemplate := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "test-tenant-id"},
		Email:               "test@example.com", Name: "Test User", IsActive: true,
	}
	must.Assert(testUserTemplate.SetPassword("Password123"))
	testUser := must.Must(factorySet.UserRegistry.Create(context.Background(), testUserTemplate))

	// Create user context and get user-aware registry set
	ctx := appctx.WithUser(context.Background(), testUser)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

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

	created1, err := registrySet.ExportRegistry.Create(ctx, export1)
	c.Assert(err, qt.IsNil)

	created2, err := registrySet.ExportRegistry.Create(ctx, export2)
	c.Assert(err, qt.IsNil)

	// Hard delete one export (changed from soft delete to be consistent with PostgreSQL)
	err = registrySet.ExportRegistry.Delete(ctx, created2.ID)
	c.Assert(err, qt.IsNil)

	// Create router with export routes
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	params := apiserver.Params{
		FactorySet:     factorySet,
		UploadLocation: "memory://",
	}
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	r.With(apiserver.RequireAuth(testJWTSecret, factorySet.UserRegistry, nil)).With(apiserver.RegistrySetMiddleware(factorySet)).Route("/exports", apiserver.Exports(params, mockRestoreWorker))

	// Test list endpoint with include_deleted=true
	req := httptest.NewRequest("GET", "/exports?include_deleted=true", nil)
	addTestUserAuthHeader(req, testUser.ID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusOK)

	var response jsonapi.ExportsResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	c.Assert(err, qt.IsNil)

	// Should return only the active export (since exports are now hard deleted)
	c.Assert(response.Data, qt.HasLen, 1)

	// Verify we only have the active export
	c.Assert(response.Data[0].ID, qt.Equals, created1.ID)
}

func TestExportCreate_SetsCreatedDate(t *testing.T) {
	c := qt.New(t)

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
		IsActive: true,
	}
	testUser.SetPassword("Password123")
	createdUser := must.Must(factorySet.UserRegistry.Create(context.Background(), testUser))

	// Create user context (registrySet not needed for this test)
	_ = appctx.WithUser(context.Background(), createdUser)

	// Create router with export routes and authentication
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(apiserver.JWTMiddleware(testJWTSecret, factorySet.UserRegistry, nil))
	r.Use(apiserver.RegistrySetMiddleware(factorySet))

	params := apiserver.Params{
		FactorySet:     factorySet,
		UploadLocation: "memory://",
		JWTSecret:      testJWTSecret,
	}
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	r.Route("/exports", apiserver.Exports(params, mockRestoreWorker))

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
	addTestUserAuthHeader(req, createdUser.ID)
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
	c.Assert(createdDateStr, qt.Contains, "T", qt.Commentf("Expected RFC3339 format with 'T', got: %s", createdDateStr))
	// RFC3339 can end with 'Z' (UTC) or timezone offset like '+02:00'
	hasValidTimezone := strings.HasSuffix(createdDateStr, "Z") || strings.Contains(createdDateStr, "+") || strings.Contains(createdDateStr, "-")
	c.Assert(hasValidTimezone, qt.IsTrue, qt.Commentf("Expected RFC3339 format with timezone, got: %s", createdDateStr))
}

// TestExportCreate_EmptyDescription_SynthesisesDefault verifies that POST
// /exports with an empty description is accepted (no 422) and that the
// service synthesises a "Backup · {type label} · {date}" default so the
// list row is never blank. Covers acceptance criterion #1 of issue #1661.
func TestExportCreate_EmptyDescription_SynthesisesDefault(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()

	testUser := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
		},
		Email:    "test+empty-desc@example.com",
		Name:     "Test User",
		IsActive: true,
	}
	testUser.SetPassword("Password123")
	createdUser := must.Must(factorySet.UserRegistry.Create(context.Background(), testUser))

	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(apiserver.JWTMiddleware(testJWTSecret, factorySet.UserRegistry, nil))
	r.Use(apiserver.RegistrySetMiddleware(factorySet))

	params := apiserver.Params{
		FactorySet:     factorySet,
		UploadLocation: "memory://",
		JWTSecret:      testJWTSecret,
	}
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	r.Route("/exports", apiserver.Exports(params, mockRestoreWorker))

	requestPayload := jsonapi.ExportCreateRequest{
		Data: &jsonapi.ExportCreateRequestData{
			Type: "exports",
			Attributes: &models.Export{
				Type:        models.ExportTypeFullDatabase,
				Description: "", // user left it blank
			},
		},
	}

	payloadBytes, err := json.Marshal(requestPayload)
	c.Assert(err, qt.IsNil)

	req := httptest.NewRequest("POST", "/exports", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	addTestUserAuthHeader(req, createdUser.ID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusCreated, qt.Commentf("body: %s", w.Body.String()))

	var response jsonapi.ExportResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	c.Assert(err, qt.IsNil)

	desc := response.Data.Attributes.Description
	c.Assert(desc, qt.Not(qt.Equals), "", qt.Commentf("expected synthesised default, got empty"))
	c.Assert(strings.HasPrefix(desc, "Backup · Full database · "), qt.IsTrue,
		qt.Commentf("expected 'Backup · Full database · …' prefix, got: %q", desc))
	// The trailing " UTC" suffix is part of the wire contract — it tells
	// the user the timestamp isn't local time. Asserted explicitly to
	// catch regressions on the format string.
	c.Assert(strings.HasSuffix(desc, " UTC"), qt.IsTrue,
		qt.Commentf("expected ' UTC' suffix, got: %q", desc))
}

// TestExportCreate_WhitespaceDescription_SynthesisesDefault verifies that
// whitespace-only descriptions are treated as empty (and replaced by the
// synthesised default). Prevents the BE from persisting a useless "   "
// description that the FE would render as a blank row.
func TestExportCreate_WhitespaceDescription_SynthesisesDefault(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()

	testUser := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
		},
		Email:    "test+whitespace-desc@example.com",
		Name:     "Test User",
		IsActive: true,
	}
	testUser.SetPassword("Password123")
	createdUser := must.Must(factorySet.UserRegistry.Create(context.Background(), testUser))

	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(apiserver.JWTMiddleware(testJWTSecret, factorySet.UserRegistry, nil))
	r.Use(apiserver.RegistrySetMiddleware(factorySet))

	params := apiserver.Params{
		FactorySet:     factorySet,
		UploadLocation: "memory://",
		JWTSecret:      testJWTSecret,
	}
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	r.Route("/exports", apiserver.Exports(params, mockRestoreWorker))

	requestPayload := jsonapi.ExportCreateRequest{
		Data: &jsonapi.ExportCreateRequestData{
			Type: "exports",
			Attributes: &models.Export{
				Type:        models.ExportTypeFullDatabase,
				Description: "   \t  ",
			},
		},
	}

	payloadBytes, err := json.Marshal(requestPayload)
	c.Assert(err, qt.IsNil)

	req := httptest.NewRequest("POST", "/exports", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	addTestUserAuthHeader(req, createdUser.ID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusCreated, qt.Commentf("body: %s", w.Body.String()))

	var response jsonapi.ExportResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	c.Assert(err, qt.IsNil)

	c.Assert(strings.HasPrefix(response.Data.Attributes.Description, "Backup · "), qt.IsTrue,
		qt.Commentf("expected 'Backup · …' prefix, got: %q", response.Data.Attributes.Description))
	c.Assert(strings.HasSuffix(response.Data.Attributes.Description, " UTC"), qt.IsTrue,
		qt.Commentf("expected ' UTC' suffix, got: %q", response.Data.Attributes.Description))
}

// TestExportCreate_LongWhitespaceDescription_SynthesisesDefault covers the
// edge case flagged in Copilot review on PR #1707: if the user submits a
// 500+ char description that's only whitespace, it should still be treated
// as blank (and replaced by the synthesised default), not 422-rejected by
// the length cap. The normalisation must happen BEFORE validation.
func TestExportCreate_LongWhitespaceDescription_SynthesisesDefault(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()

	testUser := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
		},
		Email:    "test+long-whitespace-desc@example.com",
		Name:     "Test User",
		IsActive: true,
	}
	testUser.SetPassword("Password123")
	createdUser := must.Must(factorySet.UserRegistry.Create(context.Background(), testUser))

	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(apiserver.JWTMiddleware(testJWTSecret, factorySet.UserRegistry, nil))
	r.Use(apiserver.RegistrySetMiddleware(factorySet))

	params := apiserver.Params{
		FactorySet:     factorySet,
		UploadLocation: "memory://",
		JWTSecret:      testJWTSecret,
	}
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	r.Route("/exports", apiserver.Exports(params, mockRestoreWorker))

	// 501 chars of spaces — would trip the length(0,500) cap if normalisation
	// ran after validation. The service must normalise first.
	requestPayload := jsonapi.ExportCreateRequest{
		Data: &jsonapi.ExportCreateRequestData{
			Type: "exports",
			Attributes: &models.Export{
				Type:        models.ExportTypeFullDatabase,
				Description: strings.Repeat(" ", 501),
			},
		},
	}

	payloadBytes, err := json.Marshal(requestPayload)
	c.Assert(err, qt.IsNil)

	req := httptest.NewRequest("POST", "/exports", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	addTestUserAuthHeader(req, createdUser.ID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusCreated, qt.Commentf("body: %s", w.Body.String()))

	var response jsonapi.ExportResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	c.Assert(err, qt.IsNil)

	c.Assert(strings.HasPrefix(response.Data.Attributes.Description, "Backup · "), qt.IsTrue,
		qt.Commentf("expected synthesised 'Backup · …', got: %q", response.Data.Attributes.Description))
	c.Assert(strings.HasSuffix(response.Data.Attributes.Description, " UTC"), qt.IsTrue,
		qt.Commentf("expected ' UTC' suffix, got: %q", response.Data.Attributes.Description))
}

// TestExportCreate_NonWhitespaceTooLongDescription_Rejects ensures we did NOT
// break the length cap on real (non-whitespace) descriptions. A 501-char
// non-whitespace string must still 422.
func TestExportCreate_NonWhitespaceTooLongDescription_Rejects(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()

	testUser := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
		},
		Email:    "test+too-long-desc@example.com",
		Name:     "Test User",
		IsActive: true,
	}
	testUser.SetPassword("Password123")
	createdUser := must.Must(factorySet.UserRegistry.Create(context.Background(), testUser))

	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(apiserver.JWTMiddleware(testJWTSecret, factorySet.UserRegistry, nil))
	r.Use(apiserver.RegistrySetMiddleware(factorySet))

	params := apiserver.Params{
		FactorySet:     factorySet,
		UploadLocation: "memory://",
		JWTSecret:      testJWTSecret,
	}
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	r.Route("/exports", apiserver.Exports(params, mockRestoreWorker))

	requestPayload := jsonapi.ExportCreateRequest{
		Data: &jsonapi.ExportCreateRequestData{
			Type: "exports",
			Attributes: &models.Export{
				Type:        models.ExportTypeFullDatabase,
				Description: strings.Repeat("x", 501),
			},
		},
	}

	payloadBytes, err := json.Marshal(requestPayload)
	c.Assert(err, qt.IsNil)

	req := httptest.NewRequest("POST", "/exports", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	addTestUserAuthHeader(req, createdUser.ID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusUnprocessableEntity,
		qt.Commentf("expected 422 for 501-char description, body: %s", w.Body.String()))
}

// TestImportExport_ForeignTenantSourcePath_Rejected is the cross-tenant
// security regression for POST /exports/import. A signed `.inb` archive is
// verified against a tenant-AGNOSTIC server key, so the handler MUST reject a
// SourceFilePath that lives outside the caller's own tenant namespace — without
// the guard a user could import another tenant's backup blob
// (`t/<victim>/exports/backup_….inb`) straight into their own account.
func TestImportExport_ForeignTenantSourcePath_Rejected(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()

	testUser := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "caller-tenant-id",
		},
		Email:    "test+import-foreign@example.com",
		Name:     "Test User",
		IsActive: true,
	}
	must.Assert(testUser.SetPassword("Password123"))
	createdUser := must.Must(factorySet.UserRegistry.Create(context.Background(), testUser))

	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(apiserver.JWTMiddleware(testJWTSecret, factorySet.UserRegistry, nil))
	r.Use(apiserver.RegistrySetMiddleware(factorySet))

	params := apiserver.Params{
		FactorySet:     factorySet,
		UploadLocation: "memory://",
		EntityService:  services.NewEntityService(factorySet, "memory://"),
		JWTSecret:      testJWTSecret,
	}
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	r.Route("/exports", apiserver.Exports(params, mockRestoreWorker))

	requestPayload := jsonapi.ImportExportRequest{
		Data: &jsonapi.ImportExportRequestData{
			Type: "exports",
			Attributes: &jsonapi.ImportExportAttributes{
				Description: "Import another tenant's backup",
				// Victim tenant's namespace — must be rejected.
				SourceFilePath: "t/victim-tenant-id/exports/backup_full_database_20260101.inb",
			},
		},
	}
	payloadBytes := must.Must(json.Marshal(requestPayload))

	req := httptest.NewRequest("POST", "/exports/import", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	addTestUserAuthHeader(req, createdUser.ID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusUnprocessableEntity,
		qt.Commentf("foreign-tenant import source must be rejected; body: %s", w.Body.String()))
}

// TestImportExport_OwnTenantSourcePath_Accepted is the negative control for the
// cross-tenant guard above: a legitimate restore upload is keyed
// `t/<callerTenant>/restores/...` (see blobkeys.BuildRestoreUploadKey), so the
// prefix check MUST pass and the import record must be created (201). This
// proves the guard does not break real import flows.
func TestImportExport_OwnTenantSourcePath_Accepted(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()

	testUser := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "caller-tenant-id",
		},
		Email:    "test+import-own@example.com",
		Name:     "Test User",
		IsActive: true,
	}
	must.Assert(testUser.SetPassword("Password123"))
	createdUser := must.Must(factorySet.UserRegistry.Create(context.Background(), testUser))

	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(apiserver.JWTMiddleware(testJWTSecret, factorySet.UserRegistry, nil))
	r.Use(apiserver.RegistrySetMiddleware(factorySet))

	params := apiserver.Params{
		FactorySet:     factorySet,
		UploadLocation: "memory://",
		EntityService:  services.NewEntityService(factorySet, "memory://"),
		JWTSecret:      testJWTSecret,
	}
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	r.Route("/exports", apiserver.Exports(params, mockRestoreWorker))

	requestPayload := jsonapi.ImportExportRequest{
		Data: &jsonapi.ImportExportRequestData{
			Type: "exports",
			Attributes: &jsonapi.ImportExportAttributes{
				Description: "Import my own uploaded backup",
				// Caller's own restore-upload namespace — must pass.
				SourceFilePath: "t/caller-tenant-id/restores/backup.inb",
			},
		},
	}
	payloadBytes := must.Must(json.Marshal(requestPayload))

	req := httptest.NewRequest("POST", "/exports/import", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	addTestUserAuthHeader(req, createdUser.ID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusCreated,
		qt.Commentf("own-tenant import source must be accepted; body: %s", w.Body.String()))
}

// TestGenerateExportSignedURL covers GET /exports/{id}/signed-url (#1780).
// The endpoint lets the frontend download a completed export without
// putting a JWT in the URL: it returns an HMAC-signed URL targeting the
// file-download route. Minting the URL is a side-effect-free read, so it
// is a GET (see TestGenerateExportSignedURL_NonAdminMemberAllowed for the
// authz rationale). Cases: completed export with FileID → 200 + signed
// URL; deleted, non-completed, and nil-FileID exports → 404.
func TestGenerateExportSignedURL(t *testing.T) {
	type setupResult struct {
		exportID string
		fileID   string
	}

	// makeExport seeds a file entity + an export and returns their IDs.
	// status, deleted and withFile control which 404 branch is exercised.
	makeExport := func(ctx context.Context, registrySet *registry.Set, status models.ExportStatus, deleted, withFile bool) setupResult {
		var fileID *string
		var resolvedFileID string
		if withFile {
			file := must.Must(registrySet.FileRegistry.Create(ctx, models.FileEntity{
				Title: "export-file",
				Type:  models.FileTypeDocument,
				File: &models.File{
					Path:         "exports/export",
					OriginalPath: "exports/export.xml",
					Ext:          ".xml",
					MIMEType:     "application/xml",
				},
			}))
			resolvedFileID = file.ID
			fileID = &file.ID
		}

		exp := must.Must(registrySet.ExportRegistry.Create(ctx, models.Export{
			Type:        models.ExportTypeFullDatabase,
			Description: "Signed URL test export",
			Status:      status,
			FileID:      fileID,
		}))

		if deleted {
			must.Assert(registrySet.ExportRegistry.Delete(ctx, exp.ID))
		}

		return setupResult{exportID: exp.ID, fileID: resolvedFileID}
	}

	tests := []struct {
		name           string
		status         models.ExportStatus
		deleted        bool
		withFile       bool
		expectedStatus int
	}{
		{
			name:           "completed export with file id returns signed url",
			status:         models.ExportStatusCompleted,
			withFile:       true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "deleted export returns 404",
			status:         models.ExportStatusCompleted,
			deleted:        true,
			withFile:       true,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "non-completed export returns 404",
			status:         models.ExportStatusPending,
			withFile:       true,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "export with nil file id returns 404",
			status:         models.ExportStatusCompleted,
			withFile:       false,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			factorySet := memory.NewFactorySet()

			testUser := models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "test-tenant-id"},
				Email:               "test+signed-url@example.com",
				Name:                "Test User",
				IsActive:            true,
			}
			must.Assert(testUser.SetPassword("Password123"))
			createdUser := must.Must(factorySet.UserRegistry.Create(context.Background(), testUser))

			ctx := appctx.WithUser(context.Background(), createdUser)
			registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

			seeded := makeExport(ctx, registrySet, tt.status, tt.deleted, tt.withFile)

			r := chi.NewRouter()
			r.Use(render.SetContentType(render.ContentTypeJSON))
			r.Use(apiserver.JWTMiddleware(testJWTSecret, factorySet.UserRegistry, nil))
			r.Use(apiserver.RegistrySetMiddleware(factorySet))

			params := apiserver.Params{
				FactorySet:        factorySet,
				UploadLocation:    "memory://",
				EntityService:     services.NewEntityService(factorySet, "memory://"),
				JWTSecret:         testJWTSecret,
				FileSigningKey:    []byte("test-file-signing-key-32-bytes-minimum"),
				FileURLExpiration: 15 * time.Minute,
			}
			mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
			r.Route("/exports", apiserver.Exports(params, mockRestoreWorker))

			req := httptest.NewRequest("GET", "/exports/"+seeded.exportID+"/signed-url", nil)
			addTestUserAuthHeader(req, createdUser.ID)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			c.Assert(w.Code, qt.Equals, tt.expectedStatus,
				qt.Commentf("body: %s", w.Body.String()))

			if tt.expectedStatus != http.StatusOK {
				return
			}

			var response jsonapi.SignedFileURLResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			c.Assert(err, qt.IsNil)

			c.Assert(response.ID, qt.Equals, seeded.fileID)
			c.Assert(response.Attributes.URL, qt.Not(qt.Equals), "")

			parsed, err := url.Parse(response.Attributes.URL)
			c.Assert(err, qt.IsNil)
			c.Assert(parsed.Path, qt.Equals, "/api/v1/files/download/files/"+seeded.fileID)

			query := parsed.Query()
			c.Assert(query.Get("sig"), qt.Not(qt.Equals), "")
			c.Assert(query.Get("exp"), qt.Not(qt.Equals), "")
			c.Assert(query.Get("uid"), qt.Equals, createdUser.ID)
			c.Assert(query.Get("fid"), qt.Equals, seeded.fileID)
		})
	}
}

// TestGenerateExportSignedURL_NonAdminMemberAllowed is the #1780 authz
// regression test. The /exports router is mounted behind the
// method-conditional structuralWriteGate (requireGroupRoleForWrite with
// GroupRoleAdmin): non-GET requests require admin, GET/HEAD/OPTIONS fall
// through to any group member. The signed-url endpoint is a GET precisely
// so a non-admin member can still obtain an export download URL — the same
// audience as GET /exports/{id}/download.
//
// This runs against the full apiserver.APIServer router (not a hand-wired
// sub-router) so the structuralWriteGate is genuinely in the chain. The
// test asserts both directions:
//   - a non-admin member gets 200 from GET /exports/{id}/signed-url; and
//   - the same member gets 403 from POST /exports, proving the gate is
//     actually mounted and the 200 above is meaningful, not a gate bypass.
func TestGenerateExportSignedURL_NonAdminMemberAllowed(t *testing.T) {
	c := qt.New(t)

	params, _, testGroup := newParams()

	// Create a second user and add them to the group as a non-admin
	// (viewer) member — the audience the regression locked out.
	tenantID := testGroup.TenantID
	memberTemplate := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Email:               "viewer+signed-url@example.com",
		Name:                "Viewer Member",
		IsActive:            true,
	}
	must.Assert(memberTemplate.SetPassword("Password123"))
	memberUser := must.Must(params.FactorySet.UserRegistry.Create(context.Background(), memberTemplate))
	must.Must(params.FactorySet.GroupMembershipRegistry.Create(context.Background(), models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		GroupID:             testGroup.ID,
		MemberUserID:        memberUser.ID,
		Role:                models.GroupRoleViewer,
	}))

	// Seed a completed export with a backing file entity, scoped to the
	// group, using a registry set bound to the member's user+group ctx.
	ctx := createTestUserContextWithGroup(memberUser.ID, tenantID, testGroup.ID)
	rs := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
	file := must.Must(rs.FileRegistry.Create(ctx, models.FileEntity{
		Title: "export-file",
		Type:  models.FileTypeDocument,
		File: &models.File{
			Path:         "exports/export",
			OriginalPath: "exports/export.xml",
			Ext:          ".xml",
			MIMEType:     "application/xml",
		},
	}))
	exp := must.Must(rs.ExportRegistry.Create(ctx, models.Export{
		Type:        models.ExportTypeFullDatabase,
		Description: "Signed URL test export",
		Status:      models.ExportStatusCompleted,
		FileID:      &file.ID,
	}))

	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// The non-admin member can mint a signed URL via the GET endpoint.
	rr := doJSONAPIRequest(t, handler, http.MethodGet,
		"/api/v1/g/"+testGroup.Slug+"/exports/"+exp.ID+"/signed-url", memberUser.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK,
		qt.Commentf("non-admin member must reach the signed-url GET; body: %s", rr.Body.String()))

	var response jsonapi.SignedFileURLResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &response), qt.IsNil)
	c.Assert(response.ID, qt.Equals, file.ID)
	c.Assert(response.Attributes.URL, qt.Not(qt.Equals), "")

	// Proof the structuralWriteGate is actually in the chain: the same
	// non-admin member is blocked (403) from a genuine write on the
	// gated /exports router. If this were 200/201 the test above would
	// be meaningless (gate not mounted).
	rr = doJSONAPIRequest(t, handler, http.MethodPost,
		"/api/v1/g/"+testGroup.Slug+"/exports", memberUser.ID, map[string]any{
			"data": map[string]any{
				"type": "exports",
				"attributes": map[string]any{
					"type":        string(models.ExportTypeFullDatabase),
					"description": "should be blocked for non-admin",
				},
			},
		})
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden,
		qt.Commentf("non-admin member must be blocked by structuralWriteGate on POST /exports; body: %s", rr.Body.String()))
}
