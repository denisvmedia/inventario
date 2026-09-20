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

// Group membership is a three-rung ladder — member < admin < owner — and
// every invite and membership route hangs off one of the rungs. #2114 N2
// flagged all eleven of those handlers as 0%-covered, which is an awkward
// place to have no tests: the failure mode is a viewer who can invite, or an
// admin who can delete the group, and neither looks like a bug from the
// inside.
//
// These drive the real router so the role middleware is in the path, and walk
// each rung against the routes it must not reach.

type groupsAuthzEnv struct {
	handler http.Handler
	groupID string

	ownerToken    string
	adminToken    string
	memberToken   string
	strangerToken string
	memberUserID  string
}

func newGroupsAuthzEnv(c *qt.C) *groupsAuthzEnv {
	c.Helper()

	params, owner, group := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	ctx := context.Background()

	// newParams makes the test user an admin of its group. Promote to owner
	// so the ladder has a top rung, then add one user at each other rung.
	memberships := must.Must(params.FactorySet.GroupMembershipRegistry.ListByUser(ctx, owner.TenantID, owner.ID))
	c.Assert(len(memberships) > 0, qt.IsTrue)
	ownerMembership := memberships[0]
	ownerMembership.Role = models.GroupRoleOwner
	must.Must(params.FactorySet.GroupMembershipRegistry.Update(ctx, *ownerMembership))

	join := func(email string, role models.GroupRole) *models.User {
		u := createTestUserDirect(c, params, owner.TenantID, email, true, false)
		must.Must(params.FactorySet.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: owner.TenantID},
			GroupID:             group.ID,
			MemberUserID:        u.ID,
			Role:                role,
		}))
		return u
	}

	admin := join("group-admin@example.com", models.GroupRoleAdmin)
	member := join("group-member@example.com", models.GroupRoleViewer)
	stranger := createTestUserDirect(c, params, owner.TenantID, "not-a-member@example.com", true, false)

	return &groupsAuthzEnv{
		handler:       handler,
		groupID:       group.ID,
		ownerToken:    createTestJWTToken(owner.ID),
		adminToken:    createTestJWTToken(admin.ID),
		memberToken:   createTestJWTToken(member.ID),
		strangerToken: createTestJWTToken(stranger.ID),
		memberUserID:  member.ID,
	}
}

func (e *groupsAuthzEnv) do(method, path, token string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, "/api/v1/groups/"+e.groupID+path, reader)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	e.handler.ServeHTTP(rr, req)
	return rr
}

// adminRoutes are the routes gated on admin-or-owner: member management,
// invites, and renaming the group.
func (e *groupsAuthzEnv) adminRoutes() []struct {
	name   string
	method string
	path   string
	body   any
} {
	return []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"rename group", http.MethodPatch, "", map[string]any{"name": "Renamed"}},
		{"remove member", http.MethodDelete, "/members/" + e.memberUserID, nil},
		{"change member role", http.MethodPatch, "/members/" + e.memberUserID, map[string]any{"role": "user"}},
		{"create invite", http.MethodPost, "/invites", map[string]any{"email": "x@example.com", "role": "user"}},
		{"list invites", http.MethodGet, "/invites", nil},
		{"revoke invite", http.MethodDelete, "/invites/some-invite", nil},
		{"resend invite", http.MethodPost, "/invites/some-invite/resend", nil},
	}
}

// TestGroups_ViewerCannotReachAdminRoutes is the rung that matters most: a
// viewer is the lowest-trust member, and every invite and membership route
// must be closed to them. An open one lets the least-privileged member hand
// out access.
func TestGroups_ViewerCannotReachAdminRoutes(t *testing.T) {
	c := qt.New(t)
	env := newGroupsAuthzEnv(c)

	for _, route := range env.adminRoutes() {
		c.Run(route.name, func(c *qt.C) {
			rr := env.do(route.method, route.path, env.memberToken, route.body)
			c.Assert(rr.Code, qt.Equals, http.StatusForbidden,
				qt.Commentf("a viewer reached %s %s", route.method, route.path))
		})
	}
}

// A non-member must not even see the group, let alone act on it.
func TestGroups_NonMemberCannotReachTheGroup(t *testing.T) {
	c := qt.New(t)
	env := newGroupsAuthzEnv(c)

	reads := []struct {
		name   string
		method string
		path   string
	}{
		{"get group", http.MethodGet, ""},
		{"list members", http.MethodGet, "/members"},
	}
	for _, route := range reads {
		c.Run(route.name, func(c *qt.C) {
			rr := env.do(route.method, route.path, env.strangerToken, nil)
			c.Assert(rr.Code >= 400, qt.IsTrue,
				qt.Commentf("a non-member got %d from %s %s", rr.Code, route.method, route.path))
		})
	}

	for _, route := range env.adminRoutes() {
		c.Run(route.name, func(c *qt.C) {
			rr := env.do(route.method, route.path, env.strangerToken, route.body)
			c.Assert(rr.Code >= 400, qt.IsTrue,
				qt.Commentf("a non-member got %d from %s %s", rr.Code, route.method, route.path))
		})
	}
}

// Deleting a group is the one action with no recovery, so it is owner-only
// even though admins can do everything else on this list.
func TestGroups_AdminCannotDeleteTheGroup(t *testing.T) {
	c := qt.New(t)
	env := newGroupsAuthzEnv(c)

	rr := env.do(http.MethodDelete, "", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden,
		qt.Commentf("an admin deleted a group; that rung is owner-only"))
}

// The positive side: an admin does reach the admin routes. Without this the
// tests above would pass on a surface that refuses everyone.
func TestGroups_AdminReachesAdminRoutes(t *testing.T) {
	c := qt.New(t)
	env := newGroupsAuthzEnv(c)

	rr := env.do(http.MethodGet, "/invites", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	rr = env.do(http.MethodGet, "/members", env.memberToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK,
		qt.Commentf("a member must still be able to read the member list"))
}
