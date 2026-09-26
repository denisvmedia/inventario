package apiserver_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/models"
)

// #2479 / #967 H6: ending every session for a user without disabling the
// account. Before this there was no way to do it except block-then-unblock,
// which writes two misleading audit rows and locks the user out for as long as
// the operator takes over the second call.

// revokeSessionsBody builds the request body; symmetric with unblockBody.
func revokeSessionsBody(reason string) map[string]any {
	return map[string]any{"reason": reason}
}

// seedRefreshToken gives the user a live session to revoke.
func seedRefreshToken(c *qt.C, env adminTestEnv, userID string) *models.RefreshToken {
	c.Helper()
	return must.Must(env.params.FactorySet.RefreshTokenRegistry.Create(context.Background(), models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: env.tenantID,
			UserID:   userID,
		},
		TokenHash: "hash-" + userID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}))
}

// The point of the endpoint: the sessions go and the account stays usable.
func TestAdminRevokeSessions_EndsSessionsAndLeavesTheAccountActive(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	target := createTestUserDirect(c, env.params, env.tenantID, "signed-in@example.com", true, false)
	seedRefreshToken(c, env, target.ID)

	before := must.Must(env.params.FactorySet.RefreshTokenRegistry.GetByUserID(context.Background(), target.ID))
	c.Assert(before, qt.Not(qt.HasLen), 0, qt.Commentf("the fixture has no session to revoke"))

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/"+target.ID+"/sessions/revoke",
		env.adminToken, revokeSessionsBody("laptop left logged in at a client site"))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	// The account is untouched — this is the whole difference from block.
	reloaded := must.Must(env.params.FactorySet.UserRegistry.Get(context.Background(), target.ID))
	c.Assert(reloaded.IsActive, qt.IsTrue,
		qt.Commentf("revoking sessions disabled the account; that is what block is for"))

	// And the durable half of the session is gone.
	after := must.Must(env.params.FactorySet.RefreshTokenRegistry.GetByUserID(context.Background(), target.ID))
	for _, tok := range after {
		c.Check(tok.RevokedAt, qt.IsNotNil, qt.Commentf("a refresh token survived the revoke"))
	}
}

// Calling it twice is not an error: an operator who is not sure whether the
// first call landed should be able to repeat it.
func TestAdminRevokeSessions_IsIdempotent(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	target := createTestUserDirect(c, env.params, env.tenantID, "twice@example.com", true, false)
	seedRefreshToken(c, env, target.ID)

	for range 2 {
		rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
			"/api/v1/admin/users/"+target.ID+"/sessions/revoke",
			env.adminToken, revokeSessionsBody("repeat"))
		c.Assert(rr.Code, qt.Equals, http.StatusOK)
	}
}

// A user with nothing live is also a 200 — there is no state to be wrong about.
func TestAdminRevokeSessions_NoLiveSessionsStillSucceeds(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	target := createTestUserDirect(c, env.params, env.tenantID, "never-signed-in@example.com", true, false)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/"+target.ID+"/sessions/revoke",
		env.adminToken, revokeSessionsBody("precaution"))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
}

// Another system admin needs no force flag, unlike block: ending their sessions
// does not take their access away, they sign back in.
func TestAdminRevokeSessions_NeedsNoForceForAnotherAdmin(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	peer := createTestUserDirect(c, env.params, env.tenantID, "peer-admin@example.com", true, true)
	seedRefreshToken(c, env, peer.ID)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/"+peer.ID+"/sessions/revoke",
		env.adminToken, revokeSessionsBody("suspected token leak"))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	reloaded := must.Must(env.params.FactorySet.UserRegistry.Get(context.Background(), peer.ID))
	c.Assert(reloaded.IsActive, qt.IsTrue)
}

func TestAdminRevokeSessions_UnknownUserReturns404(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/no-such-user/sessions/revoke",
		env.adminToken, revokeSessionsBody("typo"))
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

// The reason is what makes the audit row worth reading, so it is required and
// capped the same way block's is — the three endpoints answer identically to the
// same mistake.
func TestAdminRevokeSessions_BadBodyVariants(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	target := createTestUserDirect(c, env.params, env.tenantID, "body@example.com", true, false)
	path := "/api/v1/admin/users/" + target.ID + "/sessions/revoke"

	for _, tc := range []struct {
		name string
		body any
		want int
	}{
		{"missing reason", map[string]any{}, http.StatusUnprocessableEntity},
		{"blank reason", revokeSessionsBody("   "), http.StatusUnprocessableEntity},
		{"reason too long", revokeSessionsBody(strings.Repeat("x", 501)), http.StatusUnprocessableEntity},
		{"unknown field", map[string]any{"reason": "x", "force": true}, http.StatusBadRequest},
	} {
		c.Run(tc.name, func(c *qt.C) {
			rr := doAdminJSONRequest(t, env.handler, http.MethodPost, path, env.adminToken, tc.body)
			c.Assert(rr.Code, qt.Equals, tc.want)
		})
	}
}

// An unauthenticated caller must not reach it: the route is on the back-office
// plane and the middleware is the gate, but a 401 here is the assertion that it
// is actually mounted behind it.
func TestAdminRevokeSessions_RequiresBackofficeAuth(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	target := createTestUserDirect(c, env.params, env.tenantID, "nobody@example.com", true, false)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/"+target.ID+"/sessions/revoke",
		"", revokeSessionsBody("no token"))
	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}

// The audit row is the record that the sessions were ended deliberately. Its
// action is distinct from a block so a reader does not have to parse a
// breadcrumb to tell "signed out" from "disabled".
func TestAdminRevokeSessions_AuditRowIsDistinctFromBlock(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	target := createTestUserDirect(c, env.params, env.tenantID, "audited@example.com", true, false)
	seedRefreshToken(c, env, target.ID)

	const reason = "phone reported lost"
	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/"+target.ID+"/sessions/revoke",
		env.adminToken, revokeSessionsBody(reason))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	rows := must.Must(env.params.FactorySet.AuditLogRegistry.List(context.Background()))
	var found *models.AuditLog
	for _, row := range rows {
		if row.Action == apiserver.AuditActionAdminUserSessionsRevoke {
			found = row
			break
		}
	}
	c.Assert(found, qt.IsNotNil, qt.Commentf("no %s row was written", apiserver.AuditActionAdminUserSessionsRevoke))
	c.Assert(found.Success, qt.IsTrue)
	c.Assert(found.EntityID, qt.IsNotNil)
	c.Assert(*found.EntityID, qt.Equals, target.ID)
	// The reason travels in the breadcrumb the admin events share.
	c.Assert(found.UserAgent, qt.Contains, reason)

	// And nothing pretended this was a block.
	for _, row := range rows {
		c.Check(row.Action, qt.Not(qt.Equals), apiserver.AuditActionAdminUserBlock)
	}
}
