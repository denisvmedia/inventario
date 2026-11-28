package apiserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// tenantWriteEnv is the admin env plus a second tenant, "acme", with one
// signed-in user. The seeded "test-org" tenant serves every test request
// (single-tenant host resolution), so it is the one the back office runs on.
type tenantWriteEnv struct {
	adminTestEnv
	acme   *models.Tenant
	member *models.User
}

func newTenantWriteEnv(c *qt.C) tenantWriteEnv {
	c.Helper()
	env := tenantWriteEnv{adminTestEnv: newAdminEnv(c)}
	env.acme = must.Must(env.params.FactorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "Acme",
		Slug:   "acme",
		Status: models.TenantStatusActive,
	}))
	env.member = createTestUserDirect(c, env.params, env.acme.ID, "ada@acme.test", true, false)
	must.Must(env.params.FactorySet.RefreshTokenRegistry.Create(context.Background(), models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: env.acme.ID, UserID: env.member.ID},
		TokenHash:               "hash-" + env.member.ID,
		ExpiresAt:               time.Now().Add(24 * time.Hour),
	}))
	return env
}

func (e tenantWriteEnv) do(c *qt.C, method, path string, body any) *httptest.ResponseRecorder {
	c.Helper()
	return doAdminJSONRequest(c.TB.(*testing.T), e.handler, method, path, e.adminToken, body)
}

func (e tenantWriteEnv) storedTenant(c *qt.C, id string) *models.Tenant {
	c.Helper()
	return must.Must(e.params.FactorySet.TenantRegistry.Get(context.Background(), id))
}

func (e tenantWriteEnv) auditRows(c *qt.C, action string) []*models.AuditLog {
	c.Helper()
	return must.Must(e.params.FactorySet.AuditLogRegistry.ListByAction(context.Background(), action))
}

func (e tenantWriteEnv) memberSignedIn(c *qt.C) bool {
	c.Helper()
	active := must.Must(e.params.FactorySet.RefreshTokenRegistry.ListActiveByUserID(context.Background(), e.member.ID))
	return len(active) > 0
}

func decodeAdminTenant(c *qt.C, rr *httptest.ResponseRecorder) *jsonapi.AdminTenantListItem {
	c.Helper()
	var body jsonapi.AdminTenantResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil, qt.Commentf(rr.Body.String()))
	c.Assert(body.Data, qt.IsNotNil)
	return body.Data
}

func tenantsPath(suffix string) string {
	return "/api/v1/admin/tenants" + suffix
}

// Every write is closed to callers who are not platform admins: anonymous
// callers, tenant sessions (a system-admin grant included), impersonation
// sessions and support agents. None of them changes the tenant.
func TestAdminTenantWrites_RefusedForEveryoneButPlatformAdmins(t *testing.T) {
	c := qt.New(t)
	env := newTenantWriteEnv(c)

	sysadmin := createTestUserDirect(c, env.params, env.tenantID, "sysadmin@example.com", true, true)
	_, supportToken := withBackofficeOperator(t, env.params, models.BackofficeRoleSupportAgent)
	impersonationToken := makeImpersonationToken(c, impersonateClaims{
		jti:            "imp-tenant-writes",
		targetUserID:   env.member.ID,
		targetTenantID: env.acme.ID,
		operatorID:     env.admin.ID,
		expiresAt:      time.Now().Add(10 * time.Minute),
	})

	callers := []struct {
		name     string
		token    string
		wantCode int
		wantErr  string
	}{
		{"anonymous", "", http.StatusUnauthorized, ""},
		{"tenant session with a system-admin grant", createTestJWTToken(sysadmin.ID), http.StatusUnauthorized, ""},
		{"impersonation session", impersonationToken, http.StatusUnauthorized, ""},
		{"support agent", supportToken, http.StatusForbidden, apiserver.AdminRoleRequiredCode},
	}
	writes := []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, tenantsPath(""), map[string]any{"name": "Intruder"}},
		{http.MethodPatch, tenantsPath("/" + env.acme.ID), map[string]any{"name": "Renamed"}},
		{http.MethodPost, tenantsPath("/" + env.acme.ID + "/suspend"), map[string]any{"reason": "x"}},
		{http.MethodPost, tenantsPath("/" + env.acme.ID + "/reactivate"), map[string]any{"reason": "x"}},
		{http.MethodDelete, tenantsPath("/" + env.acme.ID + "?confirm_slug=acme"), nil},
	}
	for _, caller := range callers {
		for _, write := range writes {
			c.Run(caller.name+" "+write.method+" "+write.path, func(c *qt.C) {
				rr := doAdminJSONRequest(t, env.handler, write.method, write.path, caller.token, write.body)
				c.Assert(rr.Code, qt.Equals, caller.wantCode, qt.Commentf(rr.Body.String()))
				if caller.wantErr != "" {
					c.Assert(errorCode(c, rr), qt.Equals, caller.wantErr)
				}
			})
		}
	}

	stored := env.storedTenant(c, env.acme.ID)
	c.Assert(stored.Name, qt.Equals, "Acme")
	c.Assert(stored.Status, qt.Equals, models.TenantStatusActive)
	_, err := env.params.FactorySet.TenantRegistry.GetBySlug(context.Background(), "intruder")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestAdminCreateTenant(t *testing.T) {
	c := qt.New(t)
	env := newTenantWriteEnv(c)

	rr := env.do(c, http.MethodPost, tenantsPath(""), map[string]any{"name": "  Northwind Estates  ", "domain": "inventory.northwind.test"})
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf(rr.Body.String()))
	created := decodeAdminTenant(c, rr)
	c.Assert(created.Name, qt.Equals, "Northwind Estates")
	c.Assert(created.Slug, qt.Equals, "northwind-estates")
	c.Assert(created.Status, qt.Equals, models.TenantStatusActive)
	c.Assert(created.Domain, qt.DeepEquals, new("inventory.northwind.test"))
	c.Assert(created.RegistrationMode, qt.Equals, string(models.RegistrationModeClosed))

	rows := env.auditRows(c, apiserver.AuditActionAdminTenantCreate)
	c.Assert(rows, qt.HasLen, 1)
	c.Assert(rows[0].Success, qt.IsTrue)
	c.Assert(*rows[0].UserID, qt.Equals, env.admin.ID)
	c.Assert(*rows[0].EntityID, qt.Equals, created.ID)
}

func TestAdminCreateTenant_Refusals(t *testing.T) {
	tests := []struct {
		name      string
		body      any
		wantCode  int
		wantErr   string
		wantField string
	}{
		{"taken slug", map[string]any{"name": "Acme Two", "slug": "acme"}, http.StatusConflict, apiserver.TenantSlugTakenCode, ""},
		{"taken domain", map[string]any{"name": "Copycat", "domain": "inventory.acme.test"}, http.StatusConflict, apiserver.TenantDomainTakenCode, ""},
		// Single-tenant mode has no base domain, so no domain is reserved.
		{"blank name", map[string]any{"name": "   ", "slug": "blank"}, http.StatusUnprocessableEntity, "", "name"},
		{"name over 100 characters", map[string]any{"name": strings.Repeat("é", 101), "slug": "long"}, http.StatusUnprocessableEntity, "", "name"},
		{"slug with uppercase", map[string]any{"name": "Shouty", "slug": "SHOUTY"}, http.StatusUnprocessableEntity, "", "slug"},
		{"domain with a port", map[string]any{"name": "Ported", "domain": "ported.test:8080"}, http.StatusUnprocessableEntity, "", "domain"},
		{"unknown status", map[string]any{"name": "Odd", "status": "paused"}, http.StatusUnprocessableEntity, "", "status"},
		{"unknown field", map[string]any{"name": "Odd", "plan_id": "free"}, http.StatusBadRequest, "", ""},
		{"not JSON", "{", http.StatusBadRequest, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			env := newTenantWriteEnv(c)
			// The domain the "taken domain" case collides with.
			_, err := env.params.FactorySet.TenantRegistry.Update(context.Background(), func() models.Tenant {
				acme := *env.acme
				acme.Domain = new("inventory.acme.test")
				return acme
			}())
			c.Assert(err, qt.IsNil)

			rr := env.do(c, http.MethodPost, tenantsPath(""), tt.body)
			c.Assert(rr.Code, qt.Equals, tt.wantCode, qt.Commentf(rr.Body.String()))
			if tt.wantErr != "" {
				c.Assert(errorCode(c, rr), qt.Equals, tt.wantErr)
			}
			if tt.wantField != "" {
				// Field errors sit under data.attributes, where the form
				// fields are.
				c.Assert(rr.Body.String(), qt.Contains, `"attributes":{"`+tt.wantField+`"`)
			}
			if tt.wantCode != http.StatusBadRequest {
				rows := env.auditRows(c, apiserver.AuditActionAdminTenantCreate)
				c.Assert(rows, qt.HasLen, 1)
				c.Assert(rows[0].Success, qt.IsFalse)
				// The refused row says what was asked for.
				asked := tt.body.(map[string]any)
				c.Assert(rows[0].UserAgent, qt.Contains, `"name":`+string(must.Must(json.Marshal(strings.TrimSpace(asked["name"].(string))))))
			}
		})
	}
}

func TestAdminUpdateTenant(t *testing.T) {
	c := qt.New(t)
	env := newTenantWriteEnv(c)

	rr := env.do(c, http.MethodPatch, tenantsPath("/"+env.acme.ID), map[string]any{
		"name":   "Acme Inc",
		"slug":   "acme-inc",
		"domain": "inventory.acme.test",
	})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	updated := decodeAdminTenant(c, rr)
	c.Assert(updated.Name, qt.Equals, "Acme Inc")
	c.Assert(updated.Slug, qt.Equals, "acme-inc")
	c.Assert(updated.Domain, qt.DeepEquals, new("inventory.acme.test"))
	c.Assert(updated.UserCount, qt.Equals, 1)
	// A rename leaves the sessions alone.
	c.Assert(env.memberSignedIn(c), qt.IsTrue)

	// An empty domain removes it.
	rr = env.do(c, http.MethodPatch, tenantsPath("/"+env.acme.ID), map[string]any{"domain": ""})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	c.Assert(decodeAdminTenant(c, rr).Domain, qt.IsNil)

	rows := env.auditRows(c, apiserver.AuditActionAdminTenantUpdate)
	c.Assert(rows, qt.HasLen, 2)
	for _, row := range rows {
		c.Assert(row.Success, qt.IsTrue)
	}
	// Each row records the values before and after.
	c.Assert(auditRowContaining(c, rows, `"name_to":"Acme Inc"`).UserAgent, qt.Contains, `"name_from":"Acme"`)
	c.Assert(auditRowContaining(c, rows, `"domain_from":"inventory.acme.test"`).UserAgent, qt.Contains, `"domain_to":""`)

	// A body that changes nothing, empty or equal to the stored values,
	// writes nothing and records nothing.
	for _, body := range []map[string]any{{}, {"name": " Acme Inc ", "slug": "acme-inc", "domain": ""}} {
		rr = env.do(c, http.MethodPatch, tenantsPath("/"+env.acme.ID), body)
		c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
		c.Assert(decodeAdminTenant(c, rr).Name, qt.Equals, "Acme Inc")
	}
	c.Assert(env.auditRows(c, apiserver.AuditActionAdminTenantUpdate), qt.HasLen, 2)
}

// auditRowContaining returns the row whose breadcrumb contains fragment.
func auditRowContaining(c *qt.C, rows []*models.AuditLog, fragment string) *models.AuditLog {
	c.Helper()
	for _, row := range rows {
		if strings.Contains(row.UserAgent, fragment) {
			return row
		}
	}
	c.Fatalf("no audit row contains %s", fragment)
	return nil
}

// An edit that sends the domain the tenant already has is not refused as a
// taken domain. A domain another tenant has is.
func TestAdminUpdateTenant_RefusesOnlyAnotherTenantsDomain(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	env := newTenantWriteEnv(c)
	acme := *env.acme
	acme.Domain = new("acme.test")
	must.Must(env.params.FactorySet.TenantRegistry.Update(ctx, acme))
	must.Must(env.params.FactorySet.TenantRegistry.Create(ctx, models.Tenant{Name: "Other", Slug: "other", Domain: new("other.test"), Status: models.TenantStatusActive}))

	rr := env.do(c, http.MethodPatch, tenantsPath("/"+env.acme.ID), map[string]any{"name": "Acme Renamed", "domain": "acme.test"})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	c.Assert(env.storedTenant(c, env.acme.ID).Name, qt.Equals, "Acme Renamed")

	rr = env.do(c, http.MethodPatch, tenantsPath("/"+env.acme.ID), map[string]any{"domain": "other.test"})
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf(rr.Body.String()))
	c.Assert(errorCode(c, rr), qt.Equals, apiserver.TenantDomainTakenCode)
}

// multiTenantEnv serves the tenant write env with a base domain, so hosts
// under inventario.test name tenants by their slug, and sends the requests
// on the given host.
type multiTenantEnv struct {
	tenantWriteEnv
	handler http.Handler
}

const testBaseDomain = "inventario.test"

func newMultiTenantEnv(c *qt.C) multiTenantEnv {
	c.Helper()
	env := newTenantWriteEnv(c)
	params := env.params
	params.TenantResolver = &apiserver.HostTenantResolver{BaseDomain: testBaseDomain}
	return multiTenantEnv{tenantWriteEnv: env, handler: apiserver.APIServer(params, &mockRestoreWorker{})}
}

func (e multiTenantEnv) doOn(c *qt.C, host, method, path string, body any) *httptest.ResponseRecorder {
	c.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(string(must.Must(json.Marshal(body)))))
	req.Host = host
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.adminToken)
	rr := httptest.NewRecorder()
	e.handler.ServeHTTP(rr, req)
	return rr
}

// Hosts are matched against custom domains before slugs, so a custom domain
// under the base domain would take another tenant's slug host. Such domains
// are refused on create and on update.
func TestAdminTenantWrites_RefuseADomainUnderTheBaseDomain(t *testing.T) {
	c := qt.New(t)
	env := newMultiTenantEnv(c)
	host := "test-org." + testBaseDomain

	for _, domain := range []string{"acme." + testBaseDomain, testBaseDomain, "Deep.Acme.INVENTARIO.test."} {
		rr := env.doOn(c, host, http.MethodPost, tenantsPath(""), map[string]any{"name": "Shadow", "domain": domain})
		c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("%s: %s", domain, rr.Body.String()))
		c.Assert(errorCode(c, rr), qt.Equals, apiserver.TenantDomainReservedCode)
	}
	rr := env.doOn(c, host, http.MethodPatch, tenantsPath("/"+env.acme.ID), map[string]any{"domain": "www." + testBaseDomain})
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf(rr.Body.String()))
	c.Assert(errorCode(c, rr), qt.Equals, apiserver.TenantDomainReservedCode)
	c.Assert(env.storedTenant(c, env.acme.ID).Domain, qt.IsNil)

	// The refused create records the domain that was asked for.
	rows := env.auditRows(c, apiserver.AuditActionAdminTenantCreate)
	c.Assert(rows, qt.HasLen, 3)
	c.Assert(auditRowContaining(c, rows, `"domain":"acme.inventario.test"`).Success, qt.IsFalse)

	// A domain that only ends in the same letters is not under it.
	rr = env.doOn(c, host, http.MethodPost, tenantsPath(""), map[string]any{"name": "Neighbor", "domain": "my" + testBaseDomain})
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf(rr.Body.String()))
}

// The tenant serving the back-office request keeps what the request's host
// was resolved by: its slug on <slug>.<base>, its domain on its domain.
func TestAdminUpdateTenant_KeepsTheServingTenantOnItsHost(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	env := newMultiTenantEnv(c)
	slugHost := "test-org." + testBaseDomain

	rr := env.doOn(c, slugHost, http.MethodPatch, tenantsPath("/"+env.tenantID), map[string]any{"slug": "renamed"})
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf(rr.Body.String()))
	c.Assert(errorCode(c, rr), qt.Equals, apiserver.AdminTenantServesThisHostCode)
	c.Assert(env.storedTenant(c, env.tenantID).Slug, qt.Equals, "test-org")
	// Neither a new name nor a first domain moves it off the slug host.
	rr = env.doOn(c, slugHost, http.MethodPatch, tenantsPath("/"+env.tenantID), map[string]any{"name": "Renamed", "domain": "admin.example.test"})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))

	// Reached through its domain, the domain is what has to stay; the slug
	// may change.
	domainHost := "admin.example.test"
	rr = env.doOn(c, domainHost, http.MethodPatch, tenantsPath("/"+env.tenantID), map[string]any{"domain": "elsewhere.example.test"})
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf(rr.Body.String()))
	c.Assert(errorCode(c, rr), qt.Equals, apiserver.AdminTenantServesThisHostCode)
	rr = env.doOn(c, domainHost, http.MethodPatch, tenantsPath("/"+env.tenantID), map[string]any{"slug": "renamed"})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	c.Assert(must.Must(env.params.FactorySet.TenantRegistry.Get(ctx, env.tenantID)).Slug, qt.Equals, "renamed")

	// The refused rename is on record with the values it asked for.
	refused := 0
	for _, row := range env.auditRows(c, apiserver.AuditActionAdminTenantUpdate) {
		if !row.Success && strings.Contains(row.UserAgent, `"slug_from":"test-org","slug_to":"renamed"`) {
			refused++
		}
	}
	c.Assert(refused, qt.Equals, 1)
}

// In single-tenant mode every host serves the default tenant whatever its
// slug, so renaming the slug does not lock the back office out.
func TestAdminUpdateTenant_SingleTenantModeAllowsASlugChange(t *testing.T) {
	c := qt.New(t)
	env := newTenantWriteEnv(c)

	rr := env.do(c, http.MethodPatch, tenantsPath("/"+env.tenantID), map[string]any{"slug": "renamed"})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	rr = env.do(c, http.MethodGet, tenantsPath("/"+env.tenantID), nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	c.Assert(decodeAdminTenant(c, rr).Slug, qt.Equals, "renamed")
}

func TestAdminUpdateTenant_StatusAwayFromActiveEndsSessions(t *testing.T) {
	c := qt.New(t)
	env := newTenantWriteEnv(c)

	rr := env.do(c, http.MethodPatch, tenantsPath("/"+env.acme.ID), map[string]any{"status": "inactive"})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	c.Assert(decodeAdminTenant(c, rr).Status, qt.Equals, models.TenantStatusInactive)
	c.Assert(env.memberSignedIn(c), qt.IsFalse)
}

func TestAdminUpdateTenant_Refusals(t *testing.T) {
	c := qt.New(t)
	env := newTenantWriteEnv(c)

	rr := env.do(c, http.MethodPatch, tenantsPath("/"+env.acme.ID), map[string]any{"slug": "test-org"})
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf(rr.Body.String()))
	c.Assert(errorCode(c, rr), qt.Equals, apiserver.TenantSlugTakenCode)

	// Making the tenant this request is served through inactive would lock
	// the back office out.
	rr = env.do(c, http.MethodPatch, tenantsPath("/"+env.tenantID), map[string]any{"status": "suspended"})
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf(rr.Body.String()))
	c.Assert(errorCode(c, rr), qt.Equals, apiserver.AdminTenantServesThisHostCode)
	c.Assert(env.storedTenant(c, env.tenantID).Status, qt.Equals, models.TenantStatusActive)

	rr = env.do(c, http.MethodPatch, tenantsPath("/missing"), map[string]any{"name": "x"})
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound, qt.Commentf(rr.Body.String()))

	rows := env.auditRows(c, apiserver.AuditActionAdminTenantUpdate)
	c.Assert(rows, qt.HasLen, 3)
	for _, row := range rows {
		c.Assert(row.Success, qt.IsFalse)
	}
}

func TestAdminSuspendAndReactivateTenant(t *testing.T) {
	c := qt.New(t)
	env := newTenantWriteEnv(c)

	rr := env.do(c, http.MethodPost, tenantsPath("/"+env.acme.ID+"/suspend"), map[string]any{"reason": "unpaid invoice"})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	c.Assert(decodeAdminTenant(c, rr).Status, qt.Equals, models.TenantStatusSuspended)
	c.Assert(env.storedTenant(c, env.acme.ID).Status, qt.Equals, models.TenantStatusSuspended)
	c.Assert(env.memberSignedIn(c), qt.IsFalse)

	rows := env.auditRows(c, apiserver.AuditActionAdminTenantSuspend)
	c.Assert(rows, qt.HasLen, 1)
	c.Assert(rows[0].Success, qt.IsTrue)
	c.Assert(*rows[0].EntityID, qt.Equals, env.acme.ID)
	c.Assert(rows[0].UserAgent, qt.Contains, "unpaid invoice")

	// Suspending again is a 200 that changes nothing.
	rr = env.do(c, http.MethodPost, tenantsPath("/"+env.acme.ID+"/suspend"), map[string]any{"reason": "still unpaid"})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))

	rr = env.do(c, http.MethodPost, tenantsPath("/"+env.acme.ID+"/reactivate"), map[string]any{"reason": "paid"})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	c.Assert(decodeAdminTenant(c, rr).Status, qt.Equals, models.TenantStatusActive)
	// The sessions the suspension ended stay ended.
	c.Assert(env.memberSignedIn(c), qt.IsFalse)
	c.Assert(env.auditRows(c, apiserver.AuditActionAdminTenantReactivate), qt.HasLen, 1)
}

func TestAdminSuspendTenant_Refusals(t *testing.T) {
	tests := []struct {
		name     string
		target   func(env tenantWriteEnv) string
		body     any
		wantCode int
		wantErr  string
	}{
		{"no reason", func(env tenantWriteEnv) string { return env.acme.ID }, map[string]any{}, http.StatusUnprocessableEntity, apiserver.AdminTenantReasonRequiredCode},
		{"blank reason", func(env tenantWriteEnv) string { return env.acme.ID }, map[string]any{"reason": "  "}, http.StatusUnprocessableEntity, apiserver.AdminTenantReasonRequiredCode},
		{"reason over 500 characters", func(env tenantWriteEnv) string { return env.acme.ID }, map[string]any{"reason": strings.Repeat("é", 501)}, http.StatusUnprocessableEntity, apiserver.AdminTenantReasonTooLongCode},
		{"the tenant serving this host", func(env tenantWriteEnv) string { return env.tenantID }, map[string]any{"reason": "x"}, http.StatusUnprocessableEntity, apiserver.AdminTenantServesThisHostCode},
		{"unknown tenant", func(tenantWriteEnv) string { return "missing" }, map[string]any{"reason": "x"}, http.StatusNotFound, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			env := newTenantWriteEnv(c)
			rr := env.do(c, http.MethodPost, tenantsPath("/"+tt.target(env)+"/suspend"), tt.body)
			c.Assert(rr.Code, qt.Equals, tt.wantCode, qt.Commentf(rr.Body.String()))
			if tt.wantErr != "" {
				c.Assert(errorCode(c, rr), qt.Equals, tt.wantErr)
			}
			c.Assert(env.storedTenant(c, env.acme.ID).Status, qt.Equals, models.TenantStatusActive)
			c.Assert(env.storedTenant(c, env.tenantID).Status, qt.Equals, models.TenantStatusActive)
			c.Assert(env.memberSignedIn(c), qt.IsTrue)
		})
	}
}

// waitForTenantDeletion waits until the background deletion has recorded its
// outcome, and returns the audit rows of the deletion.
func waitForTenantDeletion(c *qt.C, env tenantWriteEnv, finished int) []*models.AuditLog {
	c.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		rows := env.auditRows(c, apiserver.AuditActionAdminTenantDelete)
		done := 0
		for _, row := range rows {
			if strings.Contains(row.UserAgent, `"phase":"finished"`) {
				done++
			}
		}
		if done >= finished {
			return rows
		}
		if time.Now().After(deadline) {
			c.Fatalf("the tenant deletion did not finish; audit rows: %d", len(rows))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// The delete suspends the tenant and ends its sessions, answers 202, and
// removes the tenant in the background.
func TestAdminDeleteTenant(t *testing.T) {
	c := qt.New(t)
	env := newTenantWriteEnv(c)

	rr := env.do(c, http.MethodDelete, tenantsPath("/"+env.acme.ID+"?confirm_slug=acme"), nil)
	c.Assert(rr.Code, qt.Equals, http.StatusAccepted, qt.Commentf(rr.Body.String()))
	c.Assert(decodeAdminTenant(c, rr).Status, qt.Equals, models.TenantStatusSuspended)

	rows := waitForTenantDeletion(c, env, 1)
	_, err := env.params.FactorySet.TenantRegistry.Get(context.Background(), env.acme.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = env.params.FactorySet.UserRegistry.Get(context.Background(), env.member.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// The request and the outcome are both on record, and both survive the
	// purge of the tenant's audit rows.
	c.Assert(rows, qt.HasLen, 2)
	for _, row := range rows {
		c.Assert(row.Success, qt.IsTrue)
		c.Assert(*row.EntityID, qt.Equals, env.acme.ID)
		c.Assert(row.UserAgent, qt.Contains, `"slug":"acme"`)
	}
	c.Assert(auditRowContaining(c, rows, `"phase":"accepted"`).UserAgent, qt.Contains, `"status_from":"active"`)
	c.Assert(env.auditRows(c, apiserver.AuditActionAdminTenantSuspend), qt.HasLen, 0)

	rr = env.do(c, http.MethodGet, tenantsPath("/"+env.acme.ID), nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound, qt.Commentf(rr.Body.String()))
}

// gatedPurger wraps the real purger. The dry run waits for release, fails
// while failures is positive, and counts its calls.
type gatedPurger struct {
	registry.TenantPurger
	release  chan struct{}
	failures atomic.Int32
	checks   atomic.Int32
}

func (p *gatedPurger) CheckTenantPurge(ctx context.Context, tenantID string) error {
	p.checks.Add(1)
	if p.release != nil {
		<-p.release
	}
	if p.failures.Add(-1) >= 0 {
		return errors.New("a row the purge misses")
	}
	return p.TenantPurger.CheckTenantPurge(ctx, tenantID)
}

// A deletion that fails leaves the tenant suspended with its data, records
// the failure, and continues when the delete is sent again.
func TestAdminDeleteTenant_FailureLeavesTheTenantSuspendedAndARetryContinues(t *testing.T) {
	c := qt.New(t)
	env := newTenantWriteEnv(c)
	purger := &gatedPurger{TenantPurger: env.params.FactorySet.TenantPurger}
	purger.failures.Store(1)
	env.params.FactorySet.TenantPurger = purger

	rr := env.do(c, http.MethodDelete, tenantsPath("/"+env.acme.ID+"?confirm_slug=acme"), nil)
	c.Assert(rr.Code, qt.Equals, http.StatusAccepted, qt.Commentf(rr.Body.String()))
	rows := waitForTenantDeletion(c, env, 1)
	failed := auditRowContaining(c, rows, `"phase":"finished"`)
	c.Assert(failed.Success, qt.IsFalse)
	c.Assert(*failed.ErrorMessage, qt.Contains, "a row the purge misses")
	c.Assert(env.storedTenant(c, env.acme.ID).Status, qt.Equals, models.TenantStatusSuspended)
	_, err := env.params.FactorySet.UserRegistry.Get(context.Background(), env.member.ID)
	c.Assert(err, qt.IsNil)

	rr = env.do(c, http.MethodDelete, tenantsPath("/"+env.acme.ID+"?confirm_slug=acme"), nil)
	c.Assert(rr.Code, qt.Equals, http.StatusAccepted, qt.Commentf(rr.Body.String()))
	waitForTenantDeletion(c, env, 2)
	_, err = env.params.FactorySet.TenantRegistry.Get(context.Background(), env.acme.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// A delete sent again while the deletion runs answers 202 without starting a
// second run.
func TestAdminDeleteTenant_RepeatedWhileRunning(t *testing.T) {
	c := qt.New(t)
	env := newTenantWriteEnv(c)
	purger := &gatedPurger{TenantPurger: env.params.FactorySet.TenantPurger, release: make(chan struct{})}
	env.params.FactorySet.TenantPurger = purger

	for range 2 {
		rr := env.do(c, http.MethodDelete, tenantsPath("/"+env.acme.ID+"?confirm_slug=acme"), nil)
		c.Assert(rr.Code, qt.Equals, http.StatusAccepted, qt.Commentf(rr.Body.String()))
	}
	close(purger.release)
	waitForTenantDeletion(c, env, 1)
	c.Assert(purger.checks.Load(), qt.Equals, int32(1))
	_, err := env.params.FactorySet.TenantRegistry.Get(context.Background(), env.acme.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// The confirmation is the one `inventario tenants delete` asks for: the
// slug, exactly. Anything else leaves the tenant in place.
func TestAdminDeleteTenant_Refusals(t *testing.T) {
	tests := []struct {
		name     string
		target   func(env tenantWriteEnv) string
		query    string
		wantCode int
		wantErr  string
	}{
		{"no confirmation", func(env tenantWriteEnv) string { return env.acme.ID }, "", http.StatusUnprocessableEntity, apiserver.AdminTenantConfirmationMismatchCode},
		{"another slug", func(env tenantWriteEnv) string { return env.acme.ID }, "?confirm_slug=test-org", http.StatusUnprocessableEntity, apiserver.AdminTenantConfirmationMismatchCode},
		{"another case", func(env tenantWriteEnv) string { return env.acme.ID }, "?confirm_slug=ACME", http.StatusUnprocessableEntity, apiserver.AdminTenantConfirmationMismatchCode},
		{"the tenant ID", func(env tenantWriteEnv) string { return env.acme.ID }, "?confirm_slug=" + "ID", http.StatusUnprocessableEntity, apiserver.AdminTenantConfirmationMismatchCode},
		{"the tenant serving this host", func(env tenantWriteEnv) string { return env.tenantID }, "?confirm_slug=test-org", http.StatusUnprocessableEntity, apiserver.AdminTenantServesThisHostCode},
		{"unknown tenant", func(tenantWriteEnv) string { return "missing" }, "?confirm_slug=missing", http.StatusNotFound, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			env := newTenantWriteEnv(c)
			query := strings.ReplaceAll(tt.query, "=ID", "="+env.acme.ID)
			rr := env.do(c, http.MethodDelete, tenantsPath("/"+tt.target(env)+query), nil)
			c.Assert(rr.Code, qt.Equals, tt.wantCode, qt.Commentf(rr.Body.String()))
			if tt.wantErr != "" {
				c.Assert(errorCode(c, rr), qt.Equals, tt.wantErr)
			}
			c.Assert(env.storedTenant(c, env.acme.ID).Slug, qt.Equals, "acme")
			c.Assert(env.storedTenant(c, env.tenantID).Slug, qt.Equals, "test-org")

			rows := env.auditRows(c, apiserver.AuditActionAdminTenantDelete)
			c.Assert(rows, qt.HasLen, 1)
			c.Assert(rows[0].Success, qt.IsFalse)
		})
	}
}

// Impersonating a user of a suspended tenant would hand out a session that
// every request refuses, so the start is refused instead.
func TestImpersonate_RefusesAUserOfASuspendedTenant(t *testing.T) {
	c := qt.New(t)
	env := newTenantWriteEnv(c)

	rr := env.do(c, http.MethodPost, tenantsPath("/"+env.acme.ID+"/suspend"), map[string]any{"reason": "audit"})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))

	rr = doImpersonateStart(c, env.handler, env.adminToken, env.member.ID, map[string]any{"reason": "support ticket"})
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf(rr.Body.String()))
	c.Assert(errorCode(c, rr), qt.Equals, "tenant.not_active")
}
