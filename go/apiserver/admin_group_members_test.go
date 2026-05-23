package apiserver_test

import (
	"context"
	"net/http"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/checkers"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services"
)

// Admin group-membership handler tests (#1749). Every AC in the issue
// spec maps to one (or more) of the tests below: add / remove /
// role-change, each invariant violation surfacing its sentinel, the
// cross-tenant rejection, and the audit-log breadcrumbs. The tests
// reuse the admin_users_test.go helpers (newAdminEnv,
// createTestUserDirect, doAdminJSONRequest) so the per-test setup stays
// one line.

// createAdminTestGroup creates a group directly via the registry, with
// no members. Callers seed memberships with addMembershipRow so the
// invariant tests have full control over the group's owner / member
// counts.
func createAdminTestGroup(c *qt.C, params apiserver.Params, tenantID string) *models.LocationGroup {
	c.Helper()
	slug := must.Must(models.GenerateGroupSlug())
	return must.Must(params.FactorySet.LocationGroupRegistry.Create(context.Background(), models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Name:                "Admin Test Group",
		Slug:                slug,
		Status:              models.LocationGroupStatusActive,
		GroupCurrency:       models.Currency("USD"),
	}))
}

// addMembershipRow inserts a membership row directly via the registry,
// bypassing the service so the test can construct a group with an exact
// owner / member composition.
func addMembershipRow(c *qt.C, params apiserver.Params, tenantID, groupID, userID string, role models.GroupRole) {
	c.Helper()
	must.Must(params.FactorySet.GroupMembershipRegistry.Create(context.Background(), models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		GroupID:             groupID,
		MemberUserID:        userID,
		Role:                role,
	}))
}

// findAuditRow returns the first audit row with the given action, or
// nil when none exists.
func findAuditRow(c *qt.C, params apiserver.Params, action string) *models.AuditLog {
	c.Helper()
	rows := must.Must(params.FactorySet.AuditLogRegistry.List(context.Background()))
	for i := range rows {
		if rows[i].Action == action {
			return rows[i]
		}
	}
	return nil
}

func TestAdminAddMember_HappyPath(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	group := createAdminTestGroup(c, env.params, env.tenantID)
	target := createTestUserDirect(c, env.params, env.tenantID, "newmember@example.com", true, false)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/groups/"+group.ID+"/members",
		env.adminToken, map[string]any{"userID": target.ID, "role": "user"})
	c.Assert(rr.Code, qt.Equals, http.StatusCreated)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.type"), "group_memberships")
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.group_id"), group.ID)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.member_user_id"), target.ID)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.role"), "user")

	// The membership row is persisted with a synthetic joined_at.
	m := must.Must(env.params.FactorySet.GroupMembershipRegistry.GetByGroupAndUser(context.Background(), group.ID, target.ID))
	c.Assert(m.Role, qt.Equals, models.GroupRoleUser)
	c.Assert(m.JoinedAt.IsZero(), qt.IsFalse)
}

func TestAdminAddMember_AuditRow(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	group := createAdminTestGroup(c, env.params, env.tenantID)
	target := createTestUserDirect(c, env.params, env.tenantID, "newmember@example.com", true, false)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/groups/"+group.ID+"/members",
		env.adminToken, map[string]any{"userID": target.ID, "role": "admin"})
	c.Assert(rr.Code, qt.Equals, http.StatusCreated)

	row := findAuditRow(c, env.params, apiserver.AuditActionAdminMemberAdd)
	c.Assert(row, qt.IsNotNil)
	c.Assert(row.UserID, qt.IsNotNil)
	c.Assert(*row.UserID, qt.Equals, env.admin.ID)
	c.Assert(row.EntityID, qt.IsNotNil)
	c.Assert(*row.EntityID, qt.Equals, target.ID)
	// The audit row must carry the group's tenant ID (#1749 requires
	// admin user ID + group ID + tenant ID on every membership event).
	c.Assert(row.TenantID, qt.IsNotNil)
	c.Assert(*row.TenantID, qt.Equals, group.TenantID)
	c.Assert(row.Success, qt.IsTrue)
}

func TestAdminAddMember_CrossTenantRejected(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	group := createAdminTestGroup(c, env.params, env.tenantID)

	// A user in a different tenant.
	otherTenant := must.Must(env.params.FactorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "Other Org",
		Slug:   "other-org",
		Status: models.TenantStatusActive,
	}))
	target := createTestUserDirect(c, env.params, otherTenant.ID, "outsider@example.com", true, false)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/groups/"+group.ID+"/members",
		env.adminToken, map[string]any{"userID": target.ID, "role": "user"})
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), "admin.member.tenant_mismatch")
}

func TestAdminAddMember_CapReached(t *testing.T) {
	c := qt.New(t)
	if services.MaxGroupMembershipsPerUser() <= 0 {
		c.Skip("cap disabled")
	}
	env := newAdminEnv(c)
	target := createTestUserDirect(c, env.params, env.tenantID, "capped@example.com", true, false)

	// Fill the target's membership quota up to the cap.
	for range services.MaxGroupMembershipsPerUser() {
		g := createAdminTestGroup(c, env.params, env.tenantID)
		addMembershipRow(c, env.params, env.tenantID, g.ID, target.ID, models.GroupRoleUser)
	}
	overflow := createAdminTestGroup(c, env.params, env.tenantID)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/groups/"+overflow.ID+"/members",
		env.adminToken, map[string]any{"userID": target.ID, "role": "user"})
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}

func TestAdminAddMember_AlreadyMemberRejected(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	group := createAdminTestGroup(c, env.params, env.tenantID)
	target := createTestUserDirect(c, env.params, env.tenantID, "dup@example.com", true, false)
	addMembershipRow(c, env.params, env.tenantID, group.ID, target.ID, models.GroupRoleUser)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/groups/"+group.ID+"/members",
		env.adminToken, map[string]any{"userID": target.ID, "role": "admin"})
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}

func TestAdminAddMember_InvalidRoleRejected(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	group := createAdminTestGroup(c, env.params, env.tenantID)
	target := createTestUserDirect(c, env.params, env.tenantID, "badrole@example.com", true, false)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/groups/"+group.ID+"/members",
		env.adminToken, map[string]any{"userID": target.ID, "role": "superuser"})
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), "admin.member.invalid_role")
}

func TestAdminAddMember_MissingUserIDRejected(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	group := createAdminTestGroup(c, env.params, env.tenantID)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/groups/"+group.ID+"/members",
		env.adminToken, map[string]any{"role": "user"})
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), "admin.member.user_required")
}

func TestAdminAddMember_UnknownGroupReturns404(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	target := createTestUserDirect(c, env.params, env.tenantID, "nogroup@example.com", true, false)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/groups/does-not-exist/members",
		env.adminToken, map[string]any{"userID": target.ID, "role": "user"})
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestAdminAddMember_TenantTokenRejected(t *testing.T) {
	c := qt.New(t)
	params, user, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	group := createAdminTestGroup(c, params, user.TenantID)
	target := createTestUserDirect(c, params, user.TenantID, "nope@example.com", true, false)

	// Tenant JWT — RequireBackofficeAuth rejects at the audience guard.
	rr := doAdminJSONRequest(t, handler, http.MethodPost,
		"/api/v1/admin/groups/"+group.ID+"/members",
		createTestJWTToken(user.ID), map[string]any{"userID": target.ID, "role": "user"})
	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}

func TestAdminRemoveMember_HappyPath(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	group := createAdminTestGroup(c, env.params, env.tenantID)
	owner := createTestUserDirect(c, env.params, env.tenantID, "owner@example.com", true, false)
	member := createTestUserDirect(c, env.params, env.tenantID, "member@example.com", true, false)
	addMembershipRow(c, env.params, env.tenantID, group.ID, owner.ID, models.GroupRoleOwner)
	addMembershipRow(c, env.params, env.tenantID, group.ID, member.ID, models.GroupRoleUser)

	rr := doAdminJSONRequest(t, env.handler, http.MethodDelete,
		"/api/v1/admin/groups/"+group.ID+"/members/"+member.ID,
		env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)

	_, err := env.params.FactorySet.GroupMembershipRegistry.GetByGroupAndUser(context.Background(), group.ID, member.ID)
	c.Assert(err, qt.IsNotNil)

	row := findAuditRow(c, env.params, apiserver.AuditActionAdminMemberRemove)
	c.Assert(row, qt.IsNotNil)
	c.Assert(row.TenantID, qt.IsNotNil)
	c.Assert(*row.TenantID, qt.Equals, group.TenantID)
	c.Assert(row.Success, qt.IsTrue)
}

func TestAdminRemoveMember_LastOwnerRejected(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	group := createAdminTestGroup(c, env.params, env.tenantID)
	owner := createTestUserDirect(c, env.params, env.tenantID, "soleowner@example.com", true, false)
	member := createTestUserDirect(c, env.params, env.tenantID, "plainmember@example.com", true, false)
	// One owner + one non-owner: removing the owner trips ErrLastOwner
	// (the ≥1-member invariant is still satisfied).
	addMembershipRow(c, env.params, env.tenantID, group.ID, owner.ID, models.GroupRoleOwner)
	addMembershipRow(c, env.params, env.tenantID, group.ID, member.ID, models.GroupRoleUser)

	rr := doAdminJSONRequest(t, env.handler, http.MethodDelete,
		"/api/v1/admin/groups/"+group.ID+"/members/"+owner.ID,
		env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}

func TestAdminRemoveMember_LastMemberRejected(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	group := createAdminTestGroup(c, env.params, env.tenantID)
	member := createTestUserDirect(c, env.params, env.tenantID, "lonely@example.com", true, false)
	// A single non-owner member: removing them trips ErrLastMember
	// (the owner check would pass vacuously).
	addMembershipRow(c, env.params, env.tenantID, group.ID, member.ID, models.GroupRoleUser)

	rr := doAdminJSONRequest(t, env.handler, http.MethodDelete,
		"/api/v1/admin/groups/"+group.ID+"/members/"+member.ID,
		env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), "group.last_member")
}

func TestAdminRemoveMember_NotAMemberReturns404(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	group := createAdminTestGroup(c, env.params, env.tenantID)
	stranger := createTestUserDirect(c, env.params, env.tenantID, "stranger@example.com", true, false)

	rr := doAdminJSONRequest(t, env.handler, http.MethodDelete,
		"/api/v1/admin/groups/"+group.ID+"/members/"+stranger.ID,
		env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestAdminUpdateMemberRole_HappyPath(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	group := createAdminTestGroup(c, env.params, env.tenantID)
	owner := createTestUserDirect(c, env.params, env.tenantID, "owner2@example.com", true, false)
	member := createTestUserDirect(c, env.params, env.tenantID, "promoteme@example.com", true, false)
	addMembershipRow(c, env.params, env.tenantID, group.ID, owner.ID, models.GroupRoleOwner)
	addMembershipRow(c, env.params, env.tenantID, group.ID, member.ID, models.GroupRoleUser)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPatch,
		"/api/v1/admin/groups/"+group.ID+"/members/"+member.ID,
		env.adminToken, map[string]any{"role": "admin"})
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.role"), "admin")

	m := must.Must(env.params.FactorySet.GroupMembershipRegistry.GetByGroupAndUser(context.Background(), group.ID, member.ID))
	c.Assert(m.Role, qt.Equals, models.GroupRoleAdmin)

	row := findAuditRow(c, env.params, apiserver.AuditActionAdminMemberRoleChange)
	c.Assert(row, qt.IsNotNil)
	c.Assert(row.TenantID, qt.IsNotNil)
	c.Assert(*row.TenantID, qt.Equals, group.TenantID)
	c.Assert(row.Success, qt.IsTrue)
}

func TestAdminUpdateMemberRole_LastOwnerDemotionRejected(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	group := createAdminTestGroup(c, env.params, env.tenantID)
	owner := createTestUserDirect(c, env.params, env.tenantID, "demoteme@example.com", true, false)
	member := createTestUserDirect(c, env.params, env.tenantID, "bystander@example.com", true, false)
	// Sole owner: demoting them to user would leave the group ownerless.
	addMembershipRow(c, env.params, env.tenantID, group.ID, owner.ID, models.GroupRoleOwner)
	addMembershipRow(c, env.params, env.tenantID, group.ID, member.ID, models.GroupRoleUser)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPatch,
		"/api/v1/admin/groups/"+group.ID+"/members/"+owner.ID,
		env.adminToken, map[string]any{"role": "user"})
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}

func TestAdminUpdateMemberRole_InvalidRoleRejected(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	group := createAdminTestGroup(c, env.params, env.tenantID)
	member := createTestUserDirect(c, env.params, env.tenantID, "rolecheck@example.com", true, false)
	addMembershipRow(c, env.params, env.tenantID, group.ID, member.ID, models.GroupRoleUser)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPatch,
		"/api/v1/admin/groups/"+group.ID+"/members/"+member.ID,
		env.adminToken, map[string]any{"role": "godmode"})
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), "admin.member.invalid_role")
}

func TestAdminUpdateMemberRole_NotAMemberReturns404(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	group := createAdminTestGroup(c, env.params, env.tenantID)
	stranger := createTestUserDirect(c, env.params, env.tenantID, "ghost@example.com", true, false)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPatch,
		"/api/v1/admin/groups/"+group.ID+"/members/"+stranger.ID,
		env.adminToken, map[string]any{"role": "admin"})
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

// Admin members-listing handler tests (#1756). GET
// /admin/groups/{groupID}/members backs the admin membership editor:
// it returns the group's memberships joined with each member's resolved
// identity, cross-tenant.

func TestAdminListMembers_HappyPath(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	group := createAdminTestGroup(c, env.params, env.tenantID)
	owner := createTestUserDirect(c, env.params, env.tenantID, "listowner@example.com", true, false)
	member := createTestUserDirect(c, env.params, env.tenantID, "listmember@example.com", true, false)
	addMembershipRow(c, env.params, env.tenantID, group.ID, owner.ID, models.GroupRoleOwner)
	addMembershipRow(c, env.params, env.tenantID, group.ID, member.ID, models.GroupRoleUser)

	rr := doAdminJSONRequest(t, env.handler, http.MethodGet,
		"/api/v1/admin/groups/"+group.ID+"/members", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathMatches("$.data", qt.HasLen), 2)
	// joined_at ASC ordering: the owner was seeded first.
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data[0].type"), "admin_group_members")
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data[0].member_user_id"), owner.ID)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data[0].role"), "owner")
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data[0].user.id"), owner.ID)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data[0].user.name"), "listowner@example.com")
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data[0].user.email"), "listowner@example.com")
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data[1].member_user_id"), member.ID)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data[1].role"), "user")
}

func TestAdminListMembers_UnknownGroupReturns404(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)

	rr := doAdminJSONRequest(t, env.handler, http.MethodGet,
		"/api/v1/admin/groups/does-not-exist/members", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestAdminListMembers_EmptyGroupReturns200(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	group := createAdminTestGroup(c, env.params, env.tenantID)

	// An existing group with no members is a 200 with an empty data
	// array — not a 404.
	rr := doAdminJSONRequest(t, env.handler, http.MethodGet,
		"/api/v1/admin/groups/"+group.ID+"/members", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathMatches("$.data", qt.HasLen), 0)
}

func TestAdminListMembers_CrossTenant(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)

	// A group AND its members live in a tenant other than the admin's
	// own. The RLS-bypass listing path must still surface them — this is
	// the cross-tenant proof for the #1756 admin editor.
	otherTenant := must.Must(env.params.FactorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "Other Org",
		Slug:   "other-org",
		Status: models.TenantStatusActive,
	}))
	group := createAdminTestGroup(c, env.params, otherTenant.ID)
	member := createTestUserDirect(c, env.params, otherTenant.ID, "crosstenant@example.com", true, false)
	addMembershipRow(c, env.params, otherTenant.ID, group.ID, member.ID, models.GroupRoleOwner)

	rr := doAdminJSONRequest(t, env.handler, http.MethodGet,
		"/api/v1/admin/groups/"+group.ID+"/members", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathMatches("$.data", qt.HasLen), 1)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data[0].member_user_id"), member.ID)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data[0].user.email"), "crosstenant@example.com")
}

func TestAdminListMembers_TenantTokenRejected(t *testing.T) {
	c := qt.New(t)
	params, user, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	group := createAdminTestGroup(c, params, user.TenantID)

	// Tenant JWT — RequireBackofficeAuth rejects at the audience guard.
	rr := doAdminJSONRequest(t, handler, http.MethodGet,
		"/api/v1/admin/groups/"+group.ID+"/members", createTestJWTToken(user.ID), nil)
	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}
