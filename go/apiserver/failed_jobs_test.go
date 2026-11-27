package apiserver_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/models"
)

// failedJobsEnv is one tenant with two groups (#1307): A, whose admin is
// the default test user and which has a plain member, and B, whose admin
// is another user. Each group has a failed export; A also has a failed
// restore.
type failedJobsEnv struct {
	params     apiserver.Params
	handler    http.Handler
	adminToken string
	adminA     *models.User
	member     *models.User
	adminB     *models.User
	groupA     *models.LocationGroup
	groupB     *models.LocationGroup
	exportA    string
	restoreA   string
	exportB    string
}

func newFailedJobsEnv(c *qt.C) *failedJobsEnv {
	c.Helper()
	params, adminA, groupA := newParams()
	_, adminToken := WithBackofficeAdmin(c.TB.(*testing.T), params)
	fs := params.FactorySet
	tenantID := adminA.TenantID

	newUser := func(email string) *models.User {
		u := models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
			Email:               email,
			Name:                email,
			IsActive:            true,
		}
		must.Assert(u.SetPassword("Password123"))
		return must.Must(fs.UserRegistry.Create(context.Background(), u))
	}
	member := newUser("member@example.com")
	must.Must(fs.GroupMembershipRegistry.Create(context.Background(), models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		GroupID:             groupA.ID,
		MemberUserID:        member.ID,
		Role:                models.GroupRoleUser,
	}))
	adminB := newUser("admin-b@example.com")
	groupB := createTestGroupForUser(fs, tenantID, adminB.ID)

	failedAt := time.Now().UTC().Add(-time.Hour)
	export := func(user *models.User, group *models.LocationGroup) string {
		ctx := createTestUserContextWithGroup(user.ID, tenantID, group.ID)
		set := must.Must(fs.CreateUserRegistrySet(ctx))
		e := must.Must(set.ExportRegistry.Create(ctx, models.Export{
			Type:          models.ExportTypeFullDatabase,
			Status:        models.ExportStatusFailed,
			Description:   "Weekly backup",
			ErrorMessage:  "disk full",
			CreatedDate:   models.NewPTimestamp(failedAt.Add(-time.Minute)),
			CompletedDate: models.NewPTimestamp(failedAt),
		}))
		return e.ID
	}
	ctxA := createTestUserContextWithGroup(adminA.ID, tenantID, groupA.ID)
	setA := must.Must(fs.CreateUserRegistrySet(ctxA))
	source := must.Must(setA.ExportRegistry.Create(ctxA, models.Export{
		Type:   models.ExportTypeFullDatabase,
		Status: models.ExportStatusCompleted,
	}))
	restore := must.Must(setA.RestoreOperationRegistry.Create(ctxA, models.RestoreOperation{
		ExportID:      source.ID,
		Description:   "Restore",
		Status:        models.RestoreStatusFailed,
		Options:       models.RestoreOptions{Strategy: "merge_add"},
		ErrorMessage:  "conflicting ids",
		CreatedDate:   models.NewPTimestamp(failedAt.Add(-2 * time.Hour)),
		CompletedDate: models.NewPTimestamp(failedAt.Add(-time.Minute)),
	}))

	return &failedJobsEnv{
		params:     params,
		handler:    apiserver.APIServer(params, &mockRestoreWorker{}),
		adminToken: adminToken,
		adminA:     adminA,
		member:     member,
		adminB:     adminB,
		groupA:     groupA,
		groupB:     groupB,
		exportA:    export(adminA, groupA),
		restoreA:   restore.ID,
		exportB:    export(adminB, groupB),
	}
}

func (e *failedJobsEnv) exportStatus(c *qt.C, user *models.User, group *models.LocationGroup, id string) *models.Export {
	c.Helper()
	ctx := createTestUserContextWithGroup(user.ID, user.TenantID, group.ID)
	set := must.Must(e.params.FactorySet.CreateUserRegistrySet(ctx))
	return must.Must(set.ExportRegistry.Get(ctx, id))
}

func (e *failedJobsEnv) auditRows(action string) []*models.AuditLog {
	return must.Must(e.params.FactorySet.AuditLogRegistry.ListByAction(context.Background(), action))
}

func TestAdminFailedJobs_ListsEveryTenant(t *testing.T) {
	c := qt.New(t)
	env := newFailedJobsEnv(c)

	rr := doAdminJSONRequest(t, env.handler, http.MethodGet, "/api/v1/admin/failed-jobs", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()
	c.Assert(body, checkers.JSONPathEquals("$.meta.total"), float64(3))
	c.Assert(body, checkers.JSONPathMatches("$.data", qt.HasLen), 3)
	c.Assert(body, checkers.JSONPathMatches("$.meta.job_types", qt.HasLen), len(models.FailedJobTypes()))
	c.Assert(body, checkers.JSONPathEquals("$.data[0].type"), "failed_jobs")
	c.Assert(body, checkers.JSONPathEquals("$.data[0].attributes.tenant_name"), "Test Organization")
	c.Assert(body, checkers.JSONPathEquals("$.data[0].attributes.group_name"), "Test Group")
	c.Assert(body, checkers.JSONPathEquals("$.data[0].attributes.error"), "disk full")

	rr = doAdminJSONRequest(t, env.handler, http.MethodGet, "/api/v1/admin/failed-jobs?type=restore", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathMatches("$.data", qt.HasLen), 1)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data[0].id"), env.restoreA)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data[0].attributes.retryable"), false)

	rr = doAdminJSONRequest(t, env.handler, http.MethodGet, "/api/v1/admin/failed-jobs?tenantID=another-tenant", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.meta.total"), float64(0))

	rr = doAdminJSONRequest(t, env.handler, http.MethodGet, "/api/v1/admin/failed-jobs?type=weekly-digest", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.FailedJobUnknownTypeCode)

	rows := env.auditRows(apiserver.AuditActionAdminFailedJobList)
	c.Assert(rows, qt.HasLen, 4)
	c.Assert(rows[3].Success, qt.IsFalse, qt.Commentf("a refused filter is audited too"))
}

func TestAdminFailedJobs_RetryAndDiscard(t *testing.T) {
	c := qt.New(t)
	env := newFailedJobsEnv(c)
	base := "/api/v1/admin/failed-jobs/"

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost, base+"export/"+env.exportB+"/retry", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)
	c.Assert(env.exportStatus(c, env.adminB, env.groupB, env.exportB).Status, qt.Equals, models.ExportStatusPending)
	rows := env.auditRows(apiserver.AuditActionAdminFailedJobRetry)
	c.Assert(rows, qt.HasLen, 1)
	c.Assert(rows[0].Success, qt.IsTrue)
	c.Assert(*rows[0].EntityType, qt.Equals, "export")
	c.Assert(*rows[0].EntityID, qt.Equals, env.exportB)
	c.Assert(*rows[0].TenantID, qt.Equals, env.adminB.TenantID)

	rr = doAdminJSONRequest(t, env.handler, http.MethodPost, base+"export/"+env.exportB+"/retry", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.FailedJobNotFoundCode)
	rows = env.auditRows(apiserver.AuditActionAdminFailedJobRetry)
	c.Assert(rows, qt.HasLen, 2)
	c.Assert(rows[1].Success, qt.IsFalse, qt.Commentf("a refused retry is audited too"))

	rr = doAdminJSONRequest(t, env.handler, http.MethodPost, base+"restore/"+env.restoreA+"/retry", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusConflict)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.FailedJobNotRetryableCode)

	rr = doAdminJSONRequest(t, env.handler, http.MethodPost, base+"restore/"+env.restoreA+"/discard", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)
	ctxA := createTestUserContextWithGroup(env.adminA.ID, env.adminA.TenantID, env.groupA.ID)
	setA := must.Must(env.params.FactorySet.CreateUserRegistrySet(ctxA))
	restore := must.Must(setA.RestoreOperationRegistry.Get(ctxA, env.restoreA))
	c.Assert(restore.Status, qt.Equals, models.RestoreStatusFailed, qt.Commentf("discard keeps the row"))
	c.Assert(restore.ErrorMessage, qt.Equals, "conflicting ids")
	c.Assert(restore.DiscardedAt, qt.IsNotNil)
	rows = env.auditRows(apiserver.AuditActionAdminFailedJobDiscard)
	c.Assert(rows, qt.HasLen, 1)
	c.Assert(rows[0].Success, qt.IsTrue)

	rr = doAdminJSONRequest(t, env.handler, http.MethodGet, "/api/v1/admin/failed-jobs", env.adminToken, nil)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.meta.total"), float64(1))

	rr = doAdminJSONRequest(t, env.handler, http.MethodPost, base+"weekly-digest/x/discard", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.FailedJobUnknownTypeCode)
	rows = env.auditRows(apiserver.AuditActionAdminFailedJobDiscard)
	c.Assert(rows, qt.HasLen, 2, qt.Commentf("an unknown type is refused and audited"))
	c.Assert(rows[1].Success, qt.IsFalse)
	c.Assert(*rows[1].EntityType, qt.Equals, "weekly-digest")
	c.Assert(*rows[1].EntityID, qt.Equals, "x")
}

// A support_agent may retry a failed job; discarding one needs the
// platform_admin role (#1307).
func TestAdminFailedJobs_SupportAgentRetriesButCannotDiscard(t *testing.T) {
	c := qt.New(t)
	env := newFailedJobsEnv(c)
	_, supportToken := withBackofficeOperator(t, env.params, models.BackofficeRoleSupportAgent)
	base := "/api/v1/admin/failed-jobs/"

	rr := doAdminJSONRequest(t, env.handler, http.MethodGet, "/api/v1/admin/failed-jobs", supportToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	rr = doAdminJSONRequest(t, env.handler, http.MethodPost, base+"export/"+env.exportA+"/retry", supportToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)
	c.Assert(env.exportStatus(c, env.adminA, env.groupA, env.exportA).Status, qt.Equals, models.ExportStatusPending)

	rr = doAdminJSONRequest(t, env.handler, http.MethodPost, base+"restore/"+env.restoreA+"/discard", supportToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminRoleRequiredCode)
	c.Assert(env.auditRows(apiserver.AuditActionAdminFailedJobDiscard), qt.HasLen, 0)

	rr = doAdminJSONRequest(t, env.handler, http.MethodPost, base+"restore/"+env.restoreA+"/discard", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent, qt.Commentf("a platform_admin discards"))
}

const bulkDiscardPath = "/api/v1/admin/failed-jobs/bulk-discard"

// A dry run counts the jobs a bulk discard would take and changes nothing;
// the list with the same filter shows them.
func TestAdminFailedJobs_BulkDiscardDryRun(t *testing.T) {
	c := qt.New(t)
	env := newFailedJobsEnv(c)
	now := url.QueryEscape(time.Now().UTC().Format(time.RFC3339))

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost, bulkDiscardPath+"?dry_run=true&failed_before="+now, env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()
	c.Assert(body, checkers.JSONPathEquals("$.meta.discarded"), float64(3))
	c.Assert(body, checkers.JSONPathEquals("$.meta.by_type"), map[string]any{"export": float64(2), "restore": float64(1)})
	c.Assert(body, checkers.JSONPathEquals("$.meta.dry_run"), true)
	c.Assert(env.auditRows(apiserver.AuditActionAdminFailedJobBulkDiscard), qt.HasLen, 0, qt.Commentf("a dry run is not audited"))

	rr = doAdminJSONRequest(t, env.handler, http.MethodGet, "/api/v1/admin/failed-jobs?failed_before="+now, env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.meta.total"), float64(3), qt.Commentf("nothing was discarded"))

	before := time.Now().UTC().Add(-2 * time.Hour).Format(time.DateOnly)
	rr = doAdminJSONRequest(t, env.handler, http.MethodGet, "/api/v1/admin/failed-jobs?failed_before="+before, env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.meta.total"), float64(0))

	rr = doAdminJSONRequest(t, env.handler, http.MethodGet, "/api/v1/admin/failed-jobs?failed_before=yesterday", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.FailedJobInvalidFailedBeforeCode)
}

// A bulk discard takes the jobs of the filter and writes one audit row that
// names the filter and the counts.
func TestAdminFailedJobs_BulkDiscard(t *testing.T) {
	c := qt.New(t)
	env := newFailedJobsEnv(c)
	now := time.Now().UTC().Format(time.RFC3339)
	query := url.Values{"failed_before": {now}, "type": {"export"}, "tenantID": {env.adminA.TenantID}}.Encode()

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost, bulkDiscardPath+"?"+query, env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.meta.discarded"), float64(2))
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.meta.dry_run"), false)
	c.Assert(env.exportStatus(c, env.adminA, env.groupA, env.exportA).DiscardedAt, qt.IsNotNil)
	c.Assert(env.exportStatus(c, env.adminB, env.groupB, env.exportB).DiscardedAt, qt.IsNotNil)

	rows := env.auditRows(apiserver.AuditActionAdminFailedJobBulkDiscard)
	c.Assert(rows, qt.HasLen, 1)
	c.Assert(rows[0].Success, qt.IsTrue)
	c.Assert(*rows[0].TenantID, qt.Equals, env.adminA.TenantID)
	c.Assert(*rows[0].EntityType, qt.Equals, "export")
	c.Assert(rows[0].EntityID, qt.IsNil)
	c.Assert([]byte(rows[0].UserAgent), checkers.JSONPathEquals("$.discarded"), float64(2))
	c.Assert([]byte(rows[0].UserAgent), checkers.JSONPathEquals("$.by_type"), map[string]any{"export": float64(2)})
	c.Assert([]byte(rows[0].UserAgent), checkers.JSONPathEquals("$.failed_before"), now)

	rr = doAdminJSONRequest(t, env.handler, http.MethodGet, "/api/v1/admin/failed-jobs", env.adminToken, nil)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.meta.total"), float64(1))
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data[0].id"), env.restoreA)
}

func TestAdminFailedJobs_BulkDiscardRefusesABadFilter(t *testing.T) {
	c := qt.New(t)
	env := newFailedJobsEnv(c)

	for _, tc := range []struct {
		name  string
		query url.Values
		code  string
	}{
		{"no time", url.Values{"type": {"export"}}, apiserver.FailedJobInvalidFailedBeforeCode},
		{"an unreadable time", url.Values{"failed_before": {"last week"}}, apiserver.FailedJobInvalidFailedBeforeCode},
		{"a time in the future", url.Values{"failed_before": {time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}}, apiserver.FailedJobInvalidFailedBeforeCode},
		{"an unknown type", url.Values{"failed_before": {"2026-11-01"}, "type": {"weekly-digest"}}, apiserver.FailedJobUnknownTypeCode},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			rr := doAdminJSONRequest(t, env.handler, http.MethodPost, bulkDiscardPath+"?"+tc.query.Encode(), env.adminToken, nil)
			c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)
			assertErrorCode(t, c, rr.Body.Bytes(), tc.code)
		})
	}
	rows := env.auditRows(apiserver.AuditActionAdminFailedJobBulkDiscard)
	c.Assert(rows, qt.HasLen, 4, qt.Commentf("a refused discard is audited too"))
	for _, row := range rows {
		c.Assert(row.Success, qt.IsFalse)
	}

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost, bulkDiscardPath+"?failed_before=2026-11-01&dry_run=maybe", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)

	rr = doAdminJSONRequest(t, env.handler, http.MethodGet, "/api/v1/admin/failed-jobs", env.adminToken, nil)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.meta.total"), float64(3))
}

// A bulk discard, its dry run too, needs the platform_admin role (#2946).
func TestAdminFailedJobs_SupportAgentCannotBulkDiscard(t *testing.T) {
	c := qt.New(t)
	env := newFailedJobsEnv(c)
	_, supportToken := withBackofficeOperator(t, env.params, models.BackofficeRoleSupportAgent)
	now := url.QueryEscape(time.Now().UTC().Format(time.RFC3339))

	for _, dryRun := range []string{"true", "false"} {
		rr := doAdminJSONRequest(t, env.handler, http.MethodPost, bulkDiscardPath+"?failed_before="+now+"&dry_run="+dryRun, supportToken, nil)
		c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
		assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminRoleRequiredCode)
	}
	c.Assert(env.auditRows(apiserver.AuditActionAdminFailedJobBulkDiscard), qt.HasLen, 0)
	rr := doAdminJSONRequest(t, env.handler, http.MethodGet, "/api/v1/admin/failed-jobs", env.adminToken, nil)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.meta.total"), float64(3))
}

func TestAdminFailedJobs_DeniesTenantUser(t *testing.T) {
	c := qt.New(t)
	env := newFailedJobsEnv(c)

	// A tenant token never passes the back-office gate, not even a group
	// admin's.
	rr := doJSONAPIRequest(t, env.handler, http.MethodGet, "/api/v1/admin/failed-jobs", env.adminA.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
	rr = doJSONAPIRequest(t, env.handler, http.MethodPost, "/api/v1/admin/failed-jobs/export/"+env.exportA+"/retry", env.adminA.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
	c.Assert(env.exportStatus(c, env.adminA, env.groupA, env.exportA).Status, qt.Equals, models.ExportStatusFailed)
}

func TestGroupFailedJobs_GroupAdminSeesOnlyTheGroup(t *testing.T) {
	c := qt.New(t)
	env := newFailedJobsEnv(c)

	rr := doJSONAPIRequest(t, env.handler, http.MethodGet, "/api/v1/g/"+env.groupA.Slug+"/failed-jobs", env.adminA.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()
	c.Assert(body, checkers.JSONPathEquals("$.meta.total"), float64(2))
	c.Assert(body, checkers.JSONPathMatches("$.data[*].id", qt.ContentEquals), []any{env.exportA, env.restoreA})
	c.Assert(body, checkers.JSONPathEquals("$.data[0].attributes.job_type"), "export")
	c.Assert(body, checkers.JSONPathEquals("$.data[0].attributes.retryable"), true)
	c.Assert(string(body), qt.Not(qt.Contains), "tenant_id", qt.Commentf("the group view does not name the tenant"))

	rr = doJSONAPIRequest(t, env.handler, http.MethodGet, "/api/v1/g/"+env.groupA.Slug+"/failed-jobs?type=nope", env.adminA.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)
}

func TestGroupFailedJobs_Retry(t *testing.T) {
	c := qt.New(t)
	env := newFailedJobsEnv(c)
	base := "/api/v1/g/" + env.groupA.Slug + "/failed-jobs/"

	// Another group's job is not found through this group, whatever its id.
	rr := doJSONAPIRequest(t, env.handler, http.MethodPost, base+"export/"+env.exportB+"/retry", env.adminA.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.FailedJobNotFoundCode)
	c.Assert(env.exportStatus(c, env.adminB, env.groupB, env.exportB).Status, qt.Equals, models.ExportStatusFailed)

	rr = doJSONAPIRequest(t, env.handler, http.MethodPost, base+"restore/"+env.restoreA+"/retry", env.adminA.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusConflict)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.FailedJobNotRetryableCode)

	rr = doJSONAPIRequest(t, env.handler, http.MethodPost, base+"export/"+env.exportA+"/retry", env.adminA.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)
	c.Assert(env.exportStatus(c, env.adminA, env.groupA, env.exportA).Status, qt.Equals, models.ExportStatusPending)

	rows := env.auditRows(apiserver.AuditActionFailedJobRetry)
	c.Assert(rows, qt.HasLen, 3)
	last := rows[2]
	c.Assert(last.Success, qt.IsTrue)
	c.Assert(*last.UserID, qt.Equals, env.adminA.ID)
	c.Assert(*last.EntityType, qt.Equals, "export")
	c.Assert(*last.EntityID, qt.Equals, env.exportA)
	c.Assert(rows[0].Success, qt.IsFalse, qt.Commentf("the attempt on another group's job is audited"))

	// Group admins retry; discarding is for the back office, so the group
	// plane has no such route.
	rr = doJSONAPIRequest(t, env.handler, http.MethodPost, base+"restore/"+env.restoreA+"/discard", env.adminA.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)

	rr = doJSONAPIRequest(t, env.handler, http.MethodPost, base+"weekly-digest/x/retry", env.adminA.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.FailedJobUnknownTypeCode)
	rows = env.auditRows(apiserver.AuditActionFailedJobRetry)
	c.Assert(rows, qt.HasLen, 4, qt.Commentf("an unknown type is refused and audited"))
	c.Assert(rows[3].Success, qt.IsFalse)
	c.Assert(*rows[3].EntityType, qt.Equals, "weekly-digest")
}

func TestGroupFailedJobs_RefusesMembersAndOtherGroups(t *testing.T) {
	c := qt.New(t)
	env := newFailedJobsEnv(c)
	base := "/api/v1/g/" + env.groupA.Slug + "/failed-jobs"

	rr := doJSONAPIRequest(t, env.handler, http.MethodGet, base, env.member.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf("a plain member"))
	rr = doJSONAPIRequest(t, env.handler, http.MethodPost, base+"/export/"+env.exportA+"/retry", env.member.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf("a plain member"))

	rr = doJSONAPIRequest(t, env.handler, http.MethodGet, base, env.adminB.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf("the admin of another group"))
	rr = doJSONAPIRequest(t, env.handler, http.MethodPost, base+"/export/"+env.exportA+"/retry", env.adminB.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf("the admin of another group"))

	c.Assert(env.exportStatus(c, env.adminA, env.groupA, env.exportA).Status, qt.Equals, models.ExportStatusFailed)
}
