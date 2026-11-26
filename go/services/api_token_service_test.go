package services_test

import (
	"context"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

type apiTokenServiceEnv struct {
	fs    *registry.FactorySet
	svc   *services.APITokenService
	user  *models.User
	group *models.LocationGroup
	clock time.Time
}

func newAPITokenServiceEnv(c *qt.C) *apiTokenServiceEnv {
	c.Helper()
	ctx := context.Background()
	fs := memory.NewFactorySet()

	tenant, err := fs.TenantRegistry.Create(ctx, models.Tenant{Name: "T", Slug: "t", Status: models.TenantStatusActive})
	c.Assert(err, qt.IsNil)
	user, err := fs.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Email:               "owner@example.com",
		Name:                "Owner",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)

	groupSvc := services.NewGroupService(fs.LocationGroupRegistry, fs.GroupMembershipRegistry, fs.GroupInviteRegistry)
	slug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	group, err := fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Name:                "Home",
		Slug:                slug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           user.ID,
		GroupCurrency:       "USD",
	})
	c.Assert(err, qt.IsNil)
	_, err = fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		GroupID:             group.ID,
		MemberUserID:        user.ID,
		Role:                models.GroupRoleAdmin,
	})
	c.Assert(err, qt.IsNil)

	env := &apiTokenServiceEnv{
		fs:    fs,
		svc:   services.NewAPITokenService(fs.APITokenRegistry, fs.UserRegistry, groupSvc),
		user:  user,
		group: group,
		clock: time.Now(),
	}
	env.svc.SetClock(func() time.Time { return env.clock })
	return env
}

func (e *apiTokenServiceEnv) mint(c *qt.C, in services.CreateAPITokenInput) (*models.APIToken, string) {
	c.Helper()
	if in.Name == "" {
		in.Name = "script"
	}
	if in.Scope == "" {
		in.Scope = models.APITokenScopeReadOnly
	}
	token, secret, err := e.svc.Create(context.Background(), e.user, in)
	c.Assert(err, qt.IsNil)
	return token, secret
}

func TestAPITokenService_CreateStoresOnlyTheDigest(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenServiceEnv(c)

	token, secret := env.mint(c, services.CreateAPITokenInput{Name: "  Home Assistant  "})
	c.Assert(models.IsWellFormedAPIToken(secret), qt.IsTrue)
	c.Assert(token.Name, qt.Equals, "Home Assistant")
	c.Assert(token.TokenHash, qt.Equals, models.HashAPIToken(secret))
	c.Assert(token.TokenHash, qt.Not(qt.Contains), secret[len(models.APITokenPrefix):])
	c.Assert(strings.HasPrefix(secret, token.TokenPrefix), qt.IsTrue)
	c.Assert(token.UserID, qt.Equals, env.user.ID)
	c.Assert(token.TenantID, qt.Equals, env.user.TenantID)
}

func TestAPITokenService_CreateValidation(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenServiceEnv(c)
	ctx := context.Background()

	otherTenantGroup, err := env.fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "other-tenant"},
		Name:                "Elsewhere",
		Slug:                "elsewhere-slug-123456",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           "someone",
		GroupCurrency:       "USD",
	})
	c.Assert(err, qt.IsNil)
	notMemberGroup, err := env.fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: env.user.TenantID},
		Name:                "Not mine",
		Slug:                "not-mine-slug-1234567",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           "someone",
		GroupCurrency:       "USD",
	})
	c.Assert(err, qt.IsNil)

	tests := []struct {
		name string
		in   services.CreateAPITokenInput
		want error
	}{
		{"empty name", services.CreateAPITokenInput{Name: "   ", Scope: models.APITokenScopeReadOnly}, services.ErrAPITokenNameRequired},
		{"name too long", services.CreateAPITokenInput{Name: strings.Repeat("é", services.MaxAPITokenNameLength+1), Scope: models.APITokenScopeReadOnly}, services.ErrAPITokenNameTooLong},
		{"unknown scope", services.CreateAPITokenInput{Name: "x", Scope: "admin"}, services.ErrAPITokenInvalidScope},
		{"expiry in the past", services.CreateAPITokenInput{Name: "x", Scope: models.APITokenScopeReadOnly, ExpiresAt: new(env.clock.Add(-time.Minute))}, services.ErrAPITokenExpiryInPast},
		{"expiry now", services.CreateAPITokenInput{Name: "x", Scope: models.APITokenScopeReadOnly, ExpiresAt: new(env.clock)}, services.ErrAPITokenExpiryInPast},
		{"group that does not exist", services.CreateAPITokenInput{Name: "x", Scope: models.APITokenScopeReadOnly, GroupID: new("missing")}, services.ErrAPITokenGroupNotAllowed},
		{"group in another tenant", services.CreateAPITokenInput{Name: "x", Scope: models.APITokenScopeReadOnly, GroupID: new(otherTenantGroup.ID)}, services.ErrAPITokenGroupNotAllowed},
		{"group the user is not a member of", services.CreateAPITokenInput{Name: "x", Scope: models.APITokenScopeReadOnly, GroupID: new(notMemberGroup.ID)}, services.ErrAPITokenGroupNotAllowed},
	}
	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			c.Assert(env.svc.ValidateCreate(ctx, env.user, tt.in), qt.ErrorIs, tt.want)
			_, _, err := env.svc.Create(ctx, env.user, tt.in)
			c.Assert(err, qt.ErrorIs, tt.want)
		})
	}

	list, err := env.svc.List(ctx, env.user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(list, qt.HasLen, 0, qt.Commentf("a rejected create stored a token"))
}

func TestAPITokenService_CreateWithGroupAndExpiry(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenServiceEnv(c)

	expires := env.clock.Add(30 * 24 * time.Hour)
	token, _ := env.mint(c, services.CreateAPITokenInput{
		Scope:     models.APITokenScopeReadWrite,
		GroupID:   new(env.group.ID),
		ExpiresAt: new(expires),
	})
	c.Assert(token.GroupID, qt.IsNotNil)
	c.Assert(*token.GroupID, qt.Equals, env.group.ID)
	c.Assert(token.ExpiresAt, qt.IsNotNil)
	c.Assert(token.ExpiresAt.Equal(expires), qt.IsTrue)
	c.Assert(token.Scope, qt.Equals, models.APITokenScopeReadWrite)
}

func TestAPITokenService_CreateLimit(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenServiceEnv(c)
	ctx := context.Background()

	for range services.MaxActiveAPITokensPerUser {
		env.mint(c, services.CreateAPITokenInput{})
	}
	_, _, err := env.svc.Create(ctx, env.user, services.CreateAPITokenInput{Name: "one more", Scope: models.APITokenScopeReadOnly})
	c.Assert(err, qt.ErrorIs, services.ErrAPITokenLimitReached)

	// Revoking one frees a slot.
	list, err := env.svc.List(ctx, env.user.ID)
	c.Assert(err, qt.IsNil)
	_, err = env.svc.Revoke(ctx, env.user.ID, list[0].ID)
	c.Assert(err, qt.IsNil)
	env.mint(c, services.CreateAPITokenInput{})
}

func TestAPITokenService_Authenticate(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenServiceEnv(c)
	ctx := context.Background()

	token, secret := env.mint(c, services.CreateAPITokenInput{})
	gotToken, gotUser, err := env.svc.Authenticate(ctx, secret, env.user.TenantID)
	c.Assert(err, qt.IsNil)
	c.Assert(gotToken.ID, qt.Equals, token.ID)
	c.Assert(gotUser.ID, qt.Equals, env.user.ID)

	// An empty request tenant skips only that one comparison.
	_, _, err = env.svc.Authenticate(ctx, secret, "")
	c.Assert(err, qt.IsNil)
}

// Every failure is the same error to the caller; only the attached reason,
// which goes to the server log, tells them apart.
func TestAPITokenService_AuthenticateFailures(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		setup  func(c *qt.C, env *apiTokenServiceEnv) (credential, requestTenant string)
		reason string
	}{
		{
			name: "malformed",
			setup: func(_ *qt.C, env *apiTokenServiceEnv) (string, string) {
				return "inv_pat_tooshort", env.user.TenantID
			},
			reason: services.APITokenReasonMalformed,
		},
		{
			name: "unknown",
			setup: func(c *qt.C, env *apiTokenServiceEnv) (string, string) {
				gen, err := models.GenerateAPIToken()
				c.Assert(err, qt.IsNil)
				return gen.Token, env.user.TenantID
			},
			reason: services.APITokenReasonUnknown,
		},
		{
			name: "revoked",
			setup: func(c *qt.C, env *apiTokenServiceEnv) (string, string) {
				token, secret := env.mint(c, services.CreateAPITokenInput{})
				_, err := env.svc.Revoke(ctx, env.user.ID, token.ID)
				c.Assert(err, qt.IsNil)
				return secret, env.user.TenantID
			},
			reason: services.APITokenReasonRevoked,
		},
		{
			name: "expired",
			setup: func(c *qt.C, env *apiTokenServiceEnv) (string, string) {
				_, secret := env.mint(c, services.CreateAPITokenInput{ExpiresAt: new(env.clock.Add(time.Hour))})
				env.clock = env.clock.Add(time.Hour)
				return secret, env.user.TenantID
			},
			reason: services.APITokenReasonExpired,
		},
		{
			name: "owner deactivated",
			setup: func(c *qt.C, env *apiTokenServiceEnv) (string, string) {
				_, secret := env.mint(c, services.CreateAPITokenInput{})
				u := *env.user
				u.IsActive = false
				_, err := env.fs.UserRegistry.Update(ctx, u)
				c.Assert(err, qt.IsNil)
				return secret, env.user.TenantID
			},
			reason: services.APITokenReasonUserInactive,
		},
		{
			name: "owner deleted",
			setup: func(c *qt.C, env *apiTokenServiceEnv) (string, string) {
				_, secret := env.mint(c, services.CreateAPITokenInput{})
				c.Assert(env.fs.UserRegistry.Delete(ctx, env.user.ID), qt.IsNil)
				return secret, env.user.TenantID
			},
			reason: services.APITokenReasonUserNotFound,
		},
		{
			// A token whose tenant is not its owner's, as a token would be if
			// its owner moved and the move had not deleted it.
			name: "owner in another tenant",
			setup: func(c *qt.C, env *apiTokenServiceEnv) (string, string) {
				gen, err := models.GenerateAPIToken()
				c.Assert(err, qt.IsNil)
				_, err = env.fs.APITokenRegistry.Create(ctx, models.APIToken{
					TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: "another-tenant", UserID: env.user.ID},
					Name:                    "moved",
					TokenHash:               gen.Hash,
					TokenPrefix:             gen.DisplayPrefix,
					Scope:                   models.APITokenScopeReadOnly,
				})
				c.Assert(err, qt.IsNil)
				return gen.Token, ""
			},
			reason: services.APITokenReasonTenantMismatch,
		},
		{
			name: "request addressed to another tenant",
			setup: func(c *qt.C, env *apiTokenServiceEnv) (string, string) {
				_, secret := env.mint(c, services.CreateAPITokenInput{})
				return secret, "another-tenant"
			},
			reason: services.APITokenReasonTenantMismatch,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			env := newAPITokenServiceEnv(c)
			credential, requestTenant := tt.setup(c, env)

			token, user, err := env.svc.Authenticate(ctx, credential, requestTenant)
			c.Assert(err, qt.ErrorIs, services.ErrAPITokenInvalid)
			c.Assert(services.APITokenFailureReason(err), qt.Equals, tt.reason)
			c.Assert(token, qt.IsNil)
			c.Assert(user, qt.IsNil)
		})
	}
}

func TestAPITokenService_RevokeAllForUser(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenServiceEnv(c)
	ctx := context.Background()

	_, a := env.mint(c, services.CreateAPITokenInput{})
	_, b := env.mint(c, services.CreateAPITokenInput{Scope: models.APITokenScopeReadWrite})

	n, err := env.svc.RevokeAllForUser(ctx, env.user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 2)
	for _, secret := range []string{a, b} {
		_, _, err := env.svc.Authenticate(ctx, secret, env.user.TenantID)
		c.Assert(services.APITokenFailureReason(err), qt.Equals, services.APITokenReasonRevoked)
	}
}

// RecordUse writes once per interval, so most token requests cost no write.
func TestAPITokenService_RecordUseIsThrottled(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenServiceEnv(c)
	ctx := context.Background()
	_, secret := env.mint(c, services.CreateAPITokenInput{})

	use := func() bool {
		token, _, err := env.svc.Authenticate(ctx, secret, env.user.TenantID)
		c.Assert(err, qt.IsNil)
		wrote, err := env.svc.RecordUse(ctx, token)
		c.Assert(err, qt.IsNil)
		return wrote
	}

	c.Assert(use(), qt.IsTrue, qt.Commentf("first use"))
	env.clock = env.clock.Add(time.Minute)
	c.Assert(use(), qt.IsFalse, qt.Commentf("one minute later"))
	env.clock = env.clock.Add(models.APITokenLastUsedInterval)
	c.Assert(use(), qt.IsTrue, qt.Commentf("after the interval"))

	list, err := env.svc.List(ctx, env.user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(list[0].LastUsedAt, qt.IsNotNil)
	c.Assert(list[0].LastUsedAt.Equal(env.clock), qt.IsTrue)
}
