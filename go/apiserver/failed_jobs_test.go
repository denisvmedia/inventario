package apiserver_test

import (
	"context"
	"net/http"
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

// failedPersonalExport stores a personal data export of the user whose build
// failed.
func (e *failedJobsEnv) failedPersonalExport(c *qt.C, user *models.User) string {
	c.Helper()
	ctx := context.Background()
	reg := e.params.FactorySet.PersonalDataExportRegistry
	now := time.Now().UTC()
	created, blocking, err := reg.CreateUnlessRequestedSince(ctx, models.PersonalDataExport{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: user.TenantID, UserID: user.ID},
		CreatedAt:               now.Add(-time.Hour),
		ExpiresAt:               now.Add(models.PersonalDataExportRetention),
	}, now)
	c.Assert(err, qt.IsNil)
	c.Assert(blocking, qt.IsNil)
	claimed, err := reg.ClaimPending(ctx, created.ID, now.Add(-time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)
	failed := must.Must(reg.Get(ctx, created.ID))
	failed.Status = models.PersonalDataExportStatusFailed
	failed.ErrorMessage = "the account is not active"
	failed.CompletedAt = new(now.Add(-time.Minute))
	must.Must(reg.Update(ctx, *failed))
	return created.ID
}

// A failed personal data export belongs to a user, not to a group: the back
// office lists it, retries it and discards it, with the same audit rows as
// the other types.
func TestAdminFailedJobs_PersonalDataExports(t *testing.T) {
	c := qt.New(t)
	env := newFailedJobsEnv(c)
	retried := env.failedPersonalExport(c, env.member)
	discarded := env.failedPersonalExport(c, env.adminB)
	reg := env.params.FactorySet.PersonalDataExportRegistry

	rr := doAdminJSONRequest(t, env.handler, http.MethodGet, "/api/v1/admin/failed-jobs?type=personal-data-export", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()
	c.Assert(body, checkers.JSONPathMatches("$.data[*].id", qt.ContentEquals), []any{retried, discarded})
	c.Assert(body, checkers.JSONPathMatches("$.data[*].attributes.title", qt.ContentEquals), []any{"member@example.com", "admin-b@example.com"})
	c.Assert(body, checkers.JSONPathEquals("$.data[0].attributes.job_type"), "personal-data-export")
	c.Assert(body, checkers.JSONPathEquals("$.data[0].attributes.retryable"), true)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].attributes.tenant_name"), "Test Organization")

	base := "/api/v1/admin/failed-jobs/personal-data-export/"
	rr = doAdminJSONRequest(t, env.handler, http.MethodPost, base+retried+"/retry", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)
	c.Assert(must.Must(reg.Get(context.Background(), retried)).Status, qt.Equals, models.PersonalDataExportStatusPending)
	rows := env.auditRows(apiserver.AuditActionAdminFailedJobRetry)
	c.Assert(rows, qt.HasLen, 1)
	c.Assert(rows[0].Success, qt.IsTrue)
	c.Assert(*rows[0].EntityType, qt.Equals, "personal-data-export")
	c.Assert(*rows[0].EntityID, qt.Equals, retried)
	c.Assert(*rows[0].TenantID, qt.Equals, env.member.TenantID)

	rr = doAdminJSONRequest(t, env.handler, http.MethodPost, base+discarded+"/discard", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)
	got := must.Must(reg.Get(context.Background(), discarded))
	c.Assert(got.Status, qt.Equals, models.PersonalDataExportStatusFailed, qt.Commentf("discard keeps the row"))
	c.Assert(got.DiscardedAt, qt.IsNotNil)
	rows = env.auditRows(apiserver.AuditActionAdminFailedJobDiscard)
	c.Assert(rows, qt.HasLen, 1)
	c.Assert(rows[0].Success, qt.IsTrue)
	c.Assert(*rows[0].EntityType, qt.Equals, "personal-data-export")

	rr = doAdminJSONRequest(t, env.handler, http.MethodGet, "/api/v1/admin/failed-jobs?type=personal-data-export", env.adminToken, nil)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.meta.total"), float64(0))
}

// The group card never shows a personal data export, not even the one of a
// group admin: it belongs to the user.
func TestGroupFailedJobs_LeaveOutPersonalDataExports(t *testing.T) {
	c := qt.New(t)
	env := newFailedJobsEnv(c)
	id := env.failedPersonalExport(c, env.adminA)
	base := "/api/v1/g/" + env.groupA.Slug + "/failed-jobs"

	rr := doJSONAPIRequest(t, env.handler, http.MethodGet, base, env.adminA.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()
	c.Assert(body, checkers.JSONPathMatches("$.data[*].id", qt.ContentEquals), []any{env.exportA, env.restoreA})
	c.Assert(body, checkers.JSONPathMatches("$.meta.job_types[*].type", qt.Not(qt.Contains)), "personal-data-export")

	rr = doJSONAPIRequest(t, env.handler, http.MethodGet, base+"?type=personal-data-export", env.adminA.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.FailedJobUnknownTypeCode)

	rr = doJSONAPIRequest(t, env.handler, http.MethodPost, base+"/personal-data-export/"+id+"/retry", env.adminA.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.FailedJobUnknownTypeCode)
	got := must.Must(env.params.FactorySet.PersonalDataExportRegistry.Get(context.Background(), id))
	c.Assert(got.Status, qt.Equals, models.PersonalDataExportStatusFailed)
}
