package apiserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
)

// Groups admin handler tests (#1748). Cover the cross-tenant list (filter
// combinations), the detail endpoint (tenant chip + member_count), the
// idempotent soft-delete, the system-admin gate, and the audit trail.
// Tests reuse promoteToSystemAdmin from admin_routes_test.go.

// adminGroupFixture stands up a system-admin actor plus three additional
// location groups across two tenants so the list/filter assertions have
// real data to work with. Returns the params, the admin user, and the
// IDs of the three seeded groups (the order matches the seeds slice).
func adminGroupFixture(c *qt.C) (apiserver.Params, *models.User, []string) {
	c.Helper()
	params, user, _ := newParams()
	promoteToSystemAdmin(c, params, user)

	ctx := context.Background()
	// A second tenant so the tenantID filter has something to discriminate.
	otherTenant := must.Must(params.FactorySet.TenantRegistry.Create(ctx, models.Tenant{
		Name:   "Other Org",
		Slug:   "other-org",
		Status: models.TenantStatusActive,
		PlanID: models.PlanUnlimited.ID,
	}))

	seeds := []models.LocationGroup{
		{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
			Name:                "Alpha Group",
			Slug:                must.Must(models.GenerateGroupSlug()),
			Status:              models.LocationGroupStatusActive,
			CreatedBy:           user.ID,
			GroupCurrency:       models.Currency("USD"),
		},
		{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
			Name:                "Bravo Group",
			Slug:                must.Must(models.GenerateGroupSlug()),
			Status:              models.LocationGroupStatusPendingDeletion,
			CreatedBy:           user.ID,
			GroupCurrency:       models.Currency("USD"),
		},
		{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: otherTenant.ID},
			Name:                "Charlie Group",
			Slug:                must.Must(models.GenerateGroupSlug()),
			Status:              models.LocationGroupStatusActive,
			CreatedBy:           user.ID,
			GroupCurrency:       models.Currency("EUR"),
		},
	}
	ids := make([]string, 0, len(seeds))
	for _, g := range seeds {
		created := must.Must(params.FactorySet.LocationGroupRegistry.Create(ctx, g))
		ids = append(ids, created.ID)
	}
	return params, user, ids
}

func TestAdminListGroups_AllowsSystemAdmin(t *testing.T) {
	c := qt.New(t)
	params, user, _ := adminGroupFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups", nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var body jsonapi.AdminGroupsResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
	// newParams seeds one group; the fixture adds three more.
	c.Assert(body.Meta.Total, qt.Equals, 4)
	c.Assert(body.Data, qt.HasLen, 4)
	c.Assert(rr.Header().Get("X-Total"), qt.Equals, "4")
}

func TestAdminListGroups_DeniesNonAdmin(t *testing.T) {
	c := qt.New(t)
	params, user, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups", nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
	c.Assert(rr.Body.String(), qt.Contains, "admin.forbidden")
}

func TestAdminListGroups_DeniesUnauthenticated(t *testing.T) {
	c := qt.New(t)
	params, _, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}

func TestAdminListGroups_FiltersAndPaginatesAndSorts(t *testing.T) {
	c := qt.New(t)
	params, user, _ := adminGroupFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// The fixture's "Other Org" tenant ID is needed for the tenantID
	// filter case — resolve it via the seeded group on that tenant.
	otherTenant := must.Must(params.FactorySet.TenantRegistry.GetBySlug(context.Background(), "other-org"))

	tests := []struct {
		name              string
		query             string
		expectedTotal     int
		expectedLen       int
		expectedFirstName string
	}{
		{
			name:              "filter by q narrows to one row",
			query:             "?q=alpha",
			expectedTotal:     1,
			expectedLen:       1,
			expectedFirstName: "Alpha Group",
		},
		{
			name:          "filter by status pending_deletion",
			query:         "?status=pending_deletion",
			expectedTotal: 1,
			expectedLen:   1,
		},
		{
			name:          "filter by status active",
			query:         "?status=active",
			expectedTotal: 3,
			expectedLen:   3,
		},
		{
			// ?tenantID is the documented tenant filter. The global
			// ValidateNoUserProvidedTenantID middleware would normally 403
			// a query param containing "tenant", but it exempts the
			// /api/v1/admin/* subtree — so this returns 200 and filters.
			name:          "filter by tenantID narrows to that tenant",
			query:         "?tenantID=" + otherTenant.ID,
			expectedTotal: 1,
			expectedLen:   1,
		},
		{
			name:          "combined q + status returns empty when no overlap",
			query:         "?q=alpha&status=pending_deletion",
			expectedTotal: 0,
			expectedLen:   0,
		},
		{
			name:          "pagination caps per_page",
			query:         "?per_page=2",
			expectedTotal: 4,
			expectedLen:   2,
		},
		{
			name:              "sort descending by name via dash prefix",
			query:             "?sort=-name",
			expectedTotal:     4,
			expectedLen:       4,
			expectedFirstName: "Test Group", // seeded by newParams, sorts last asc → first desc
		},
		{
			name:              "sort by name ascending via explicit order",
			query:             "?sort=name&order=asc",
			expectedTotal:     4,
			expectedLen:       4,
			expectedFirstName: "Alpha Group",
		},
	}
	for _, tc := range tests {
		c.Run(tc.name, func(c *qt.C) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups"+tc.query, nil)
			addTestUserAuthHeader(req, user.ID)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			c.Assert(rr.Code, qt.Equals, http.StatusOK)
			var body jsonapi.AdminGroupsResponse
			c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
			c.Assert(body.Meta.Total, qt.Equals, tc.expectedTotal)
			c.Assert(body.Data, qt.HasLen, tc.expectedLen)
			if tc.expectedFirstName != "" {
				c.Assert(body.Data[0].Name, qt.Equals, tc.expectedFirstName)
			}
		})
	}
}

func TestAdminGetGroup_ReturnsRowWithTenantChipAndMemberCount(t *testing.T) {
	c := qt.New(t)
	params, user, _ := newParams()
	promoteToSystemAdmin(c, params, user)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// newParams seeds exactly one group on the actor's tenant with one
	// membership (the creator). Resolve it for the detail assertion.
	groups := must.Must(params.FactorySet.LocationGroupRegistry.ListByTenant(context.Background(), user.TenantID))
	c.Assert(groups, qt.HasLen, 1)
	group := groups[0]

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/groups/%s", group.ID), nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	var body jsonapi.AdminGroupResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
	c.Assert(body.Data, qt.IsNotNil)
	c.Assert(body.Data.ID, qt.Equals, group.ID)
	c.Assert(body.Data.TenantID, qt.Equals, user.TenantID)
	c.Assert(body.Data.MemberCount, qt.Equals, 1)
	c.Assert(body.Data.Tenant, qt.IsNotNil)
	c.Assert(body.Data.Tenant.ID, qt.Equals, user.TenantID)
}

func TestAdminGetGroup_404OnMissingID(t *testing.T) {
	c := qt.New(t)
	params, user, _ := adminGroupFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/does-not-exist", nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestAdminGetGroup_DeniesNonAdmin(t *testing.T) {
	c := qt.New(t)
	params, user, ids := adminGroupFixture(c)
	// Demote: adminGroupFixture promotes, so build a separate non-admin.
	nonAdmin := createTestUserDirect(c, params, user.TenantID, "plain@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/groups/%s", ids[0]), nil)
	addTestUserAuthHeader(req, nonAdmin.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
}

func TestAdminDeleteGroup_MarksPendingDeletion(t *testing.T) {
	c := qt.New(t)
	params, user, ids := adminGroupFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// ids[0] is the active "Alpha Group".
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/groups/%s", ids[0]), nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	var body jsonapi.AdminGroupResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
	c.Assert(body.Data, qt.IsNotNil)
	c.Assert(body.Data.Status, qt.Equals, models.LocationGroupStatusPendingDeletion)

	// And the registry row reflects the transition.
	stored := must.Must(params.FactorySet.LocationGroupRegistry.Get(context.Background(), ids[0]))
	c.Assert(stored.Status, qt.Equals, models.LocationGroupStatusPendingDeletion)
}

func TestAdminDeleteGroup_IdempotentReDelete(t *testing.T) {
	c := qt.New(t)
	params, user, ids := adminGroupFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// ids[1] is the "Bravo Group" seeded already in pending_deletion —
	// a re-delete must still return 200 with the current status, never
	// a 4xx.
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/groups/%s", ids[1]), nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	var body jsonapi.AdminGroupResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
	c.Assert(body.Data.Status, qt.Equals, models.LocationGroupStatusPendingDeletion)
}

func TestAdminDeleteGroup_404OnMissingID(t *testing.T) {
	c := qt.New(t)
	params, user, _ := adminGroupFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/groups/does-not-exist", nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestAdminDeleteGroup_DeniesNonAdmin(t *testing.T) {
	c := qt.New(t)
	params, user, ids := adminGroupFixture(c)
	nonAdmin := createTestUserDirect(c, params, user.TenantID, "plain@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/groups/%s", ids[0]), nil)
	addTestUserAuthHeader(req, nonAdmin.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)

	// The guard rejected before the transition could fire.
	stored := must.Must(params.FactorySet.LocationGroupRegistry.Get(context.Background(), ids[0]))
	c.Assert(stored.Status, qt.Equals, models.LocationGroupStatusActive)
}

func TestAdminListGroups_WritesAuditLog(t *testing.T) {
	c := qt.New(t)
	params, user, _ := adminGroupFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups", nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	entries := must.Must(params.FactorySet.AuditLogRegistry.ListByAction(context.Background(), "admin.list_groups"))
	c.Assert(entries, qt.HasLen, 1)
	c.Assert(entries[0].Success, qt.IsTrue)
	c.Assert(entries[0].UserID, qt.IsNotNil)
	c.Assert(*entries[0].UserID, qt.Equals, user.ID)
}

func TestAdminDeleteGroup_WritesAuditLogWithActorGroupAndTenant(t *testing.T) {
	c := qt.New(t)
	params, user, ids := adminGroupFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/groups/%s", ids[0]), nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	entries := must.Must(params.FactorySet.AuditLogRegistry.ListByAction(context.Background(), "admin.delete_group"))
	c.Assert(entries, qt.HasLen, 1)
	entry := entries[0]
	c.Assert(entry.Success, qt.IsTrue)
	// Admin user ID (actor).
	c.Assert(entry.UserID, qt.IsNotNil)
	c.Assert(*entry.UserID, qt.Equals, user.ID)
	// Group ID (subject).
	c.Assert(entry.EntityID, qt.IsNotNil)
	c.Assert(*entry.EntityID, qt.Equals, ids[0])
	c.Assert(entry.EntityType, qt.IsNotNil)
	c.Assert(*entry.EntityType, qt.Equals, "group")
	// Tenant ID of the group.
	c.Assert(entry.TenantID, qt.IsNotNil)
	c.Assert(*entry.TenantID, qt.Equals, user.TenantID)
}
