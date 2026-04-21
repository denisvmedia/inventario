package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

const inviteTestTenantID = testTenantID

// inviteFixture wires up a memory-backed GroupService with a single group
// owned by a creator user, and exposes helpers to mint invites of various
// shapes (valid, expired, pre-used). All data lives in inviteTestTenantID.
type inviteFixture struct {
	groupService *services.GroupService
	group        *models.LocationGroup
	groupReg     *memory.LocationGroupRegistry
	membership   *memory.GroupMembershipRegistry
	invites      *memory.GroupInviteRegistry
	creatorID    string
}

func newInviteFixture(c *qt.C) *inviteFixture {
	groupReg := memory.NewLocationGroupRegistry()
	members := memory.NewGroupMembershipRegistry()
	invites := memory.NewGroupInviteRegistry()
	svc := services.NewGroupService(groupReg, members, invites)

	creatorID := "creator-user"
	group, err := svc.CreateGroup(context.Background(), inviteTestTenantID, creatorID, "Test Group", "", models.Currency("USD"))
	c.Assert(err, qt.IsNil)

	return &inviteFixture{
		groupService: svc,
		group:        group,
		groupReg:     groupReg,
		membership:   members,
		invites:      invites,
		creatorID:    creatorID,
	}
}

// mintInvite returns a fresh single-use token with default 24h expiry.
func (f *inviteFixture) mintInvite(c *qt.C) string {
	invite, err := f.groupService.CreateInvite(context.Background(), inviteTestTenantID, f.group.ID, f.creatorID, 0)
	c.Assert(err, qt.IsNil)
	return invite.Token
}

// mintExpiredInvite produces a token whose ExpiresAt is already in the past.
// It creates the invite via the service (which forbids past expiry) and then
// back-dates it directly via the registry.
func (f *inviteFixture) mintExpiredInvite(c *qt.C) string {
	invite, err := f.groupService.CreateInvite(context.Background(), inviteTestTenantID, f.group.ID, f.creatorID, 0)
	c.Assert(err, qt.IsNil)
	invite.ExpiresAt = time.Now().Add(-1 * time.Hour)
	_, err = f.invites.Update(context.Background(), *invite)
	c.Assert(err, qt.IsNil)
	return invite.Token
}

// mintUsedInvite produces a token that has already been consumed.
func (f *inviteFixture) mintUsedInvite(c *qt.C) string {
	invite, err := f.groupService.CreateInvite(context.Background(), inviteTestTenantID, f.group.ID, f.creatorID, 0)
	c.Assert(err, qt.IsNil)
	won, err := f.invites.MarkUsed(context.Background(), invite.ID, "some-other-user", time.Now())
	c.Assert(err, qt.IsNil)
	c.Assert(won, qt.IsTrue)
	return invite.Token
}

// newRegistrationRouterWithInvites mirrors newRegistrationRouter but injects
// a GroupService so the handler can validate invite_token.
func newRegistrationRouterWithInvites(params apiserver.RegistrationParams) chi.Router {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(apiserver.WithTenantID(req.Context(), inviteTestTenantID)))
		})
	})
	r.Group(apiserver.Registration(params))
	return r
}

func postRegister(c *qt.C, r chi.Router, payload map[string]string) *httptest.ResponseRecorder {
	body, err := json.Marshal(payload)
	c.Assert(err, qt.IsNil)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// closedModeNoInvite — the baseline gate: closed mode + no invite ⇒ 403.
func TestHandleRegister_ClosedModeNoInviteReturns403(t *testing.T) {
	c := qt.New(t)

	f := newInviteFixture(c)
	userReg := &registrationUserRegistry{mockUserRegistryForAuth: &mockUserRegistryForAuth{users: map[string]*models.User{}}}
	r := newRegistrationRouterWithInvites(apiserver.RegistrationParams{
		UserRegistry:         userReg,
		VerificationRegistry: memory.NewEmailVerificationRegistry(),
		GroupService:         f.groupService,
		RegistrationMode:     models.RegistrationModeClosed,
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	})

	w := postRegister(c, r, map[string]string{
		"email":    "no-invite@example.com",
		"name":     "No Invite",
		"password": "Password123",
	})
	c.Assert(w.Code, qt.Equals, http.StatusForbidden)
	c.Assert(userReg.users, qt.HasLen, 0, qt.Commentf("no user should be created when registration is closed"))
}

// closedModeValidInvite — the invite bypass path: user is created ACTIVE
// and no verification email is sent (tested by asserting IsActive=true and
// that no welcome/verification hook was invoked — we just check IsActive
// and the 200 status; the dedicated blocking-email test covers the
// async-send path when verification *should* fire).
func TestHandleRegister_ClosedModeValidInviteCreatesActiveUser(t *testing.T) {
	c := qt.New(t)

	f := newInviteFixture(c)
	userReg := &registrationUserRegistry{mockUserRegistryForAuth: &mockUserRegistryForAuth{users: map[string]*models.User{}}}
	emailSvc := &blockingEmailService{
		verificationCh: make(chan asyncEmailObservation, 1),
	}
	r := newRegistrationRouterWithInvites(apiserver.RegistrationParams{
		UserRegistry:         userReg,
		VerificationRegistry: memory.NewEmailVerificationRegistry(),
		EmailService:         emailSvc,
		GroupService:         f.groupService,
		RegistrationMode:     models.RegistrationModeClosed,
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	})

	token := f.mintInvite(c)
	w := postRegister(c, r, map[string]string{
		"email":        "invitee@example.com",
		"name":         "Invitee",
		"password":     "Password123",
		"invite_token": token,
	})

	c.Assert(w.Code, qt.Equals, http.StatusOK)
	c.Assert(userReg.users, qt.HasLen, 1)

	var created *models.User
	for _, u := range userReg.users {
		created = u
	}
	c.Assert(created, qt.IsNotNil)
	c.Assert(created.Email, qt.Equals, "invitee@example.com")
	c.Assert(created.IsActive, qt.IsTrue, qt.Commentf("invite-based registration must mark user active immediately"))

	// No verification email should fire on the invite path. We check two
	// signals for that: (1) the atomic call counter on the email stub stays
	// at zero; (2) nothing appears on verificationCh within a generous
	// grace window. Relying on the channel alone is racey when the
	// assertion timeout is tight — the counter makes the assertion
	// deterministic regardless of goroutine scheduling. The 500ms window
	// matches the cancellation-path test's budget so a slow CI worker
	// doesn't flake this one into a false pass.
	select {
	case <-emailSvc.verificationCh:
		t.Fatal("no verification email should be sent on invite-based registration")
	case <-time.After(500 * time.Millisecond):
		// expected
	}
	c.Assert(emailSvc.verificationCalls.Load(), qt.Equals, int32(0),
		qt.Commentf("invite-based registration must never invoke SendVerificationEmail"))

	// The invite itself is NOT consumed by registration; the caller must
	// POST /invites/{token}/accept after logging in. Verify by looking up
	// the invite directly.
	invite, err := f.invites.GetByToken(context.Background(), token)
	c.Assert(err, qt.IsNil)
	c.Assert(invite.IsUsed(), qt.IsFalse, qt.Commentf("registration must not consume the invite"))
}

// closedModeExpiredInvite — 400 with a distinguishable message.
func TestHandleRegister_ClosedModeExpiredInviteReturns400(t *testing.T) {
	c := qt.New(t)

	f := newInviteFixture(c)
	userReg := &registrationUserRegistry{mockUserRegistryForAuth: &mockUserRegistryForAuth{users: map[string]*models.User{}}}
	r := newRegistrationRouterWithInvites(apiserver.RegistrationParams{
		UserRegistry:         userReg,
		VerificationRegistry: memory.NewEmailVerificationRegistry(),
		GroupService:         f.groupService,
		RegistrationMode:     models.RegistrationModeClosed,
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	})

	token := f.mintExpiredInvite(c)
	w := postRegister(c, r, map[string]string{
		"email":        "expired@example.com",
		"name":         "Expired",
		"password":     "Password123",
		"invite_token": token,
	})

	c.Assert(w.Code, qt.Equals, http.StatusBadRequest)
	c.Assert(w.Body.String(), qt.Contains, "expired")
	c.Assert(userReg.users, qt.HasLen, 0)
}

// closedModeUsedInvite — 400 with a distinguishable message.
func TestHandleRegister_ClosedModeUsedInviteReturns400(t *testing.T) {
	c := qt.New(t)

	f := newInviteFixture(c)
	userReg := &registrationUserRegistry{mockUserRegistryForAuth: &mockUserRegistryForAuth{users: map[string]*models.User{}}}
	r := newRegistrationRouterWithInvites(apiserver.RegistrationParams{
		UserRegistry:         userReg,
		VerificationRegistry: memory.NewEmailVerificationRegistry(),
		GroupService:         f.groupService,
		RegistrationMode:     models.RegistrationModeClosed,
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	})

	token := f.mintUsedInvite(c)
	w := postRegister(c, r, map[string]string{
		"email":        "used@example.com",
		"name":         "Used",
		"password":     "Password123",
		"invite_token": token,
	})

	c.Assert(w.Code, qt.Equals, http.StatusBadRequest)
	c.Assert(w.Body.String(), qt.Contains, "used")
	c.Assert(userReg.users, qt.HasLen, 0)
}

// closedModeUnknownInvite — 400, "invalid" (never leak whether the token
// belonged to another tenant).
func TestHandleRegister_ClosedModeUnknownInviteReturns400(t *testing.T) {
	c := qt.New(t)

	f := newInviteFixture(c)
	userReg := &registrationUserRegistry{mockUserRegistryForAuth: &mockUserRegistryForAuth{users: map[string]*models.User{}}}
	r := newRegistrationRouterWithInvites(apiserver.RegistrationParams{
		UserRegistry:         userReg,
		VerificationRegistry: memory.NewEmailVerificationRegistry(),
		GroupService:         f.groupService,
		RegistrationMode:     models.RegistrationModeClosed,
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	})

	w := postRegister(c, r, map[string]string{
		"email":        "unknown@example.com",
		"name":         "Unknown",
		"password":     "Password123",
		"invite_token": "this-token-does-not-exist",
	})

	c.Assert(w.Code, qt.Equals, http.StatusBadRequest)
	c.Assert(w.Body.String(), qt.Contains, "invalid")
	c.Assert(userReg.users, qt.HasLen, 0)
}

// openMode still ignores the invite token when unset — no regression of the
// happy registration path.
func TestHandleRegister_OpenModeWithoutInviteStillWorks(t *testing.T) {
	c := qt.New(t)

	f := newInviteFixture(c)
	userReg := &registrationUserRegistry{mockUserRegistryForAuth: &mockUserRegistryForAuth{users: map[string]*models.User{}}}
	r := newRegistrationRouterWithInvites(apiserver.RegistrationParams{
		UserRegistry:         userReg,
		VerificationRegistry: memory.NewEmailVerificationRegistry(),
		GroupService:         f.groupService,
		RegistrationMode:     models.RegistrationModeOpen,
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	})

	w := postRegister(c, r, map[string]string{
		"email":    "open@example.com",
		"name":     "Open",
		"password": "Password123",
	})
	c.Assert(w.Code, qt.Equals, http.StatusOK)
	c.Assert(userReg.users, qt.HasLen, 1)

	var created *models.User
	for _, u := range userReg.users {
		created = u
	}
	c.Assert(created.IsActive, qt.IsFalse, qt.Commentf("open-mode registration without invite must stay pending email verification"))
}
