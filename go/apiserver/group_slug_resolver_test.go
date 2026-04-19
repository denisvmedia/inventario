package apiserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/golang-jwt/jwt/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
)

// TestGroupSlugResolver_ValidMember asserts that a member of an active group
// can reach a group-scoped data endpoint (200).
func TestGroupSlugResolver_ValidMember(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()

	req := must.Must(http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+"/locations", nil))
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()
	apiserver.APIServer(params, &mockRestoreWorker{}).ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
}

// TestGroupSlugResolver_UnknownSlug asserts that a slug that does not resolve
// to any group in the user's tenant is rejected with 404 (Not Found).
func TestGroupSlugResolver_UnknownSlug(t *testing.T) {
	c := qt.New(t)

	params, testUser, _ := newParams()

	req := must.Must(http.NewRequest("GET", "/api/v1/g/this-slug-does-not-exist/locations", nil))
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()
	apiserver.APIServer(params, &mockRestoreWorker{}).ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

// TestGroupSlugResolver_NonMember asserts that a slug that resolves to an
// active group, but where the requesting user is NOT a member, is rejected
// with 403 (Forbidden).
//
// NOTE on the information-disclosure trade-off: returning 403 here does reveal
// the existence of the group to an authenticated user — unlike a masked 404,
// which is what we use for cross-group invite lookups (see
// NewMaskedNotFoundError / services.ErrInviteNotInGroup in
// go/apiserver/errors.go). The trade-offs diverge intentionally:
//
//   - Slugs are 22+ chars of base64url (random). A non-member typically only
//     knows a slug because someone shared the invite link. Distinguishing "no
//     such group" from "you're not in it" lets the UI tell the user to ask
//     for an invite rather than staring at a 404.
//   - Invite IDs, by contrast, are DB-internal. Leaking cross-group invite
//     existence via the DELETE path would let a group admin probe invite IDs
//     belonging to other groups in the same tenant — no legitimate UX need.
//
// If the product decision changes and group slugs become guessable (shorter,
// or picked by the user), flip this to a masked 404 and update the test.
func TestGroupSlugResolver_NonMember(t *testing.T) {
	c := qt.New(t)

	params, testUser, _ := newParams()

	// Create a second user in the same tenant and give them a separate group.
	// testUser has a membership for their own group (via newParams) but not
	// for this second group.
	otherUser := must.Must(params.FactorySet.UserRegistry.Create(context.Background(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: testUser.TenantID,
		},
		Email:    "other@example.com",
		Name:     "Other User",
		IsActive: true,
	}))
	otherGroup := createTestGroupForUser(params.FactorySet, testUser.TenantID, otherUser.ID)

	req := must.Must(http.NewRequest("GET", "/api/v1/g/"+otherGroup.Slug+"/locations", nil))
	addTestUserAuthHeader(req, testUser.ID) // testUser, not otherUser
	rr := httptest.NewRecorder()
	apiserver.APIServer(params, &mockRestoreWorker{}).ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
}

// TestGroupSlugResolver_InactiveGroup asserts that a slug resolving to a
// group in pending_deletion state returns 410 (Gone) — even for its own
// admin. This prevents an admin from continuing to write data into a group
// that is scheduled for removal.
func TestGroupSlugResolver_InactiveGroup(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()

	// Flip the group to pending_deletion directly in the registry.
	testGroup.Status = models.LocationGroupStatusPendingDeletion
	must.Must(params.FactorySet.LocationGroupRegistry.Update(context.Background(), *testGroup))

	req := must.Must(http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+"/locations", nil))
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()
	apiserver.APIServer(params, &mockRestoreWorker{}).ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusGone)
}

// TestGroupSlugResolver_Unauthenticated asserts that an un-authenticated
// request to a group-scoped route is rejected with 401 by the JWT middleware
// before the slug resolver runs.
func TestGroupSlugResolver_Unauthenticated(t *testing.T) {
	c := qt.New(t)

	params, _, testGroup := newParams()

	req := must.Must(http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+"/locations", nil))
	// No auth header.
	rr := httptest.NewRecorder()
	apiserver.APIServer(params, &mockRestoreWorker{}).ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}

// TestGroupSlugResolver_StaleUserToken asserts that a JWT whose user id no
// longer exists is rejected (the JWT middleware runs before slug resolution,
// so this checks that our group-scoped mount didn't accidentally open a hole
// around that guard).
func TestGroupSlugResolver_StaleUserToken(t *testing.T) {
	c := qt.New(t)

	params, _, testGroup := newParams()

	// Build a JWT for a user that does not exist in the registry.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": "user-does-not-exist",
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenString := must.Must(token.SignedString(testJWTSecret))

	req := must.Must(http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+"/locations", nil))
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rr := httptest.NewRecorder()
	apiserver.APIServer(params, &mockRestoreWorker{}).ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}
