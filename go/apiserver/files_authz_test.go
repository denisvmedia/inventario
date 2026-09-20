package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/models"
)

// The file surface is where "authorization" and "a bug nobody notices" meet:
// a leak here answers with somebody else's invoice rather than an error, so
// nothing in the logs looks wrong. #2114 N2 flagged every handler in files.go
// as 0%-covered. These tests drive the real router — group-slug resolution,
// membership check, RLS-scoped registries and all — because the isolation
// lives in that chain rather than in the handlers.

// filesAuthzEnv is one deployment holding two tenants that must never see
// each other, plus a second group inside the FIRST tenant. Same-tenant,
// different-group is the interesting case: RLS cannot separate those, so the
// membership check is the only thing standing between them.
type filesAuthzEnv struct {
	handler http.Handler
	params  apiserver.Params

	ownerToken   string
	ownerGroup   *models.LocationGroup
	ownerFileID  string
	strangerTok  string
	strangerSlug string
	// otherGroup belongs to the SAME tenant as the owner but a different
	// user, so a request for it passes the tenant boundary and has to be
	// stopped by membership alone.
	otherGroupSlug string
	otherGroupTok  string
}

func newFilesAuthzEnv(c *qt.C) *filesAuthzEnv {
	c.Helper()

	params, owner, ownerGroup := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// A file that really exists, in the owner's group.
	ownerCtx := createTestUserContextWithGroup(owner.ID, owner.TenantID, ownerGroup.ID)
	ownerSet := must.Must(params.FactorySet.CreateUserRegistrySet(ownerCtx))
	file := must.Must(ownerSet.FileRegistry.Create(ownerCtx, models.FileEntity{
		Title:    "private-invoice",
		Type:     models.FileTypeDocument,
		Category: models.FileCategoryDocuments,
		Tags:     []string{},
		File: &models.File{
			Path:         "private-invoice",
			OriginalPath: "private-invoice.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	// A second tenant entirely.
	otherTenant := must.Must(params.FactorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name: "Other Organization", Slug: "other-org", Status: models.TenantStatusActive,
	}))
	stranger := createTestUserDirect(c, params, otherTenant.ID, "stranger@other.example", true, false)
	strangerGroup := createTestGroupForUser(params.FactorySet, otherTenant.ID, stranger.ID)

	// A second user + group inside the OWNER's tenant.
	neighbour := createTestUserDirect(c, params, owner.TenantID, "neighbour@example.com", true, false)
	neighbourGroup := createTestGroupForUser(params.FactorySet, owner.TenantID, neighbour.ID)

	return &filesAuthzEnv{
		handler:        handler,
		params:         params,
		ownerToken:     createTestJWTToken(owner.ID),
		ownerGroup:     ownerGroup,
		ownerFileID:    file.ID,
		strangerTok:    createTestJWTToken(stranger.ID),
		strangerSlug:   strangerGroup.Slug,
		otherGroupSlug: neighbourGroup.Slug,
		otherGroupTok:  createTestJWTToken(neighbour.ID),
	}
}

func (e *filesAuthzEnv) do(t *testing.T, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	e.handler.ServeHTTP(rr, req)
	return rr
}

func (e *filesAuthzEnv) groupPath(slug, suffix string) string {
	return "/api/v1/g/" + slug + "/files" + suffix
}

// TestFiles_CrossGroupAccessIsRefused walks every read and delete route on the
// file surface from two directions that must both fail:
//
//   - a user in ANOTHER TENANT naming the owner's group slug;
//   - a user in the SAME TENANT naming a group they do not belong to.
//
// The second is the one worth having: RLS separates tenants for free, so a
// same-tenant cross-group leak is the failure that would actually survive
// review.
func TestFiles_CrossGroupAccessIsRefused(t *testing.T) {
	c := qt.New(t)
	env := newFilesAuthzEnv(c)

	routes := []struct {
		name   string
		method string
		suffix string
	}{
		{"get one", http.MethodGet, "/" + env.ownerFileID},
		{"delete", http.MethodDelete, "/" + env.ownerFileID},
		{"list", http.MethodGet, ""},
		{"category counts", http.MethodGet, "/category-counts"},
	}

	callers := []struct {
		name  string
		token string
	}{
		{"a user from another tenant", env.strangerTok},
		{"a user from the same tenant, different group", env.otherGroupTok},
	}

	for _, route := range routes {
		for _, caller := range callers {
			c.Run(route.name+" / "+caller.name, func(c *qt.C) {
				rr := env.do(t, route.method, env.groupPath(env.ownerGroup.Slug, route.suffix), caller.token)
				// The group-slug resolver refuses a non-member before the
				// handler runs. 403 or 404 are both acceptable answers —
				// what must never happen is a 2xx.
				c.Assert(rr.Code >= 400, qt.IsTrue,
					qt.Commentf("%s %s answered %d for %s",
						route.method, route.suffix, rr.Code, caller.name))
			})
		}
	}
}

// The owner's own file is reachable — otherwise the test above would pass on
// a surface that is simply broken for everyone.
func TestFiles_OwnerCanReadOwnFile(t *testing.T) {
	c := qt.New(t)
	env := newFilesAuthzEnv(c)

	rr := env.do(t, http.MethodGet, env.groupPath(env.ownerGroup.Slug, "/"+env.ownerFileID), env.ownerToken)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.String(), qt.Contains, "private-invoice")
}

// A file id from another group must read as absent inside a group the caller
// DOES belong to — the caller is authorized for this group, so the answer has
// to come from the scoping, not from the membership check.
func TestFiles_ForeignFileIDIsNotFoundInOwnGroup(t *testing.T) {
	c := qt.New(t)
	env := newFilesAuthzEnv(c)

	rr := env.do(t, http.MethodGet,
		env.groupPath(env.strangerSlug, "/"+env.ownerFileID), env.strangerTok)
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound,
		qt.Commentf("a foreign id inside your own group must be absent, not readable"))
}

// Bulk delete takes a list of ids. A caller must not be able to smuggle one
// belonging to another group into an otherwise-legitimate request.
func TestFiles_BulkDeleteIgnoresForeignIDs(t *testing.T) {
	c := qt.New(t)
	env := newFilesAuthzEnv(c)

	body := must.Must(json.Marshal(map[string]any{
		"ids": []string{env.ownerFileID},
	}))
	req := httptest.NewRequest(http.MethodPost,
		env.groupPath(env.strangerSlug, "/bulk-delete"), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+env.strangerTok)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)

	// Whatever the endpoint answers, the owner's file must still be there.
	ownerCtx := createTestUserContextWithGroup(
		env.ownerGroup.CreatedBy, env.ownerGroup.TenantID, env.ownerGroup.ID)
	set := must.Must(env.params.FactorySet.CreateUserRegistrySet(ownerCtx))
	survived, err := set.FileRegistry.Get(ownerCtx, env.ownerFileID)
	c.Assert(err, qt.IsNil, qt.Commentf("bulk delete answered %d and took the file with it", rr.Code))
	c.Assert(survived.Title, qt.Equals, "private-invoice")
}

// An unauthenticated request never reaches the handler.
func TestFiles_AnonymousIsRejected(t *testing.T) {
	c := qt.New(t)
	env := newFilesAuthzEnv(c)

	req := httptest.NewRequest(http.MethodGet,
		env.groupPath(env.ownerGroup.Slug, "/"+env.ownerFileID), nil)
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}
