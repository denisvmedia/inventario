package admin_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
	"go.5x5.cz/inventario/services/admin"
)

func TestGenerateTenantSlug(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"words", "Acme Corp", "acme-corp"},
		{"punctuation runs collapse and the ends are trimmed", "  --Hello, World!!  ", "hello-world"},
		{"letters outside ASCII are separators", "Café Ünïcode", "caf-n-code"},
		{"digits stay", "Team 42", "team-42"},
		{"cut at 50 characters", strings.Repeat("a", 60), strings.Repeat("a", 50)},
		{"a cut that ends on a hyphen drops it", strings.Repeat("a", 49) + " bcd", strings.Repeat("a", 49)},
		{"nothing usable gives an empty slug", "!!!", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(admin.GenerateTenantSlug(tt.in), qt.Equals, tt.want)
		})
	}
}

// The confirmation is exact: the CLI prompt and the back office both ask for
// the slug as it is.
func TestConfirmTenantDeletion(t *testing.T) {
	tenant := &models.Tenant{EntityID: models.EntityID{ID: "t1"}, Slug: "acme"}
	tests := []struct {
		name  string
		typed string
		ok    bool
	}{
		{"the slug", "acme", true},
		{"empty", "", false},
		{"another case", "ACME", false},
		{"surrounding spaces", " acme ", false},
		{"the tenant ID", "t1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			err := admin.ConfirmTenantDeletion(tenant, tt.typed)
			if tt.ok {
				c.Assert(err, qt.IsNil)
				return
			}
			c.Assert(err, qt.ErrorIs, admin.ErrTenantDeleteNotConfirmed)
		})
	}
}

func TestService_CreateTenant_FillsTheDefaults(t *testing.T) {
	c := qt.New(t)
	svc := admin.NewServiceWithFactorySet(memory.NewFactorySet())

	created, err := svc.CreateTenant(context.Background(), admin.TenantCreateRequest{Name: "Northwind Estates"})
	c.Assert(err, qt.IsNil)
	c.Assert(created.Slug, qt.Equals, "northwind-estates")
	c.Assert(created.Status, qt.Equals, models.TenantStatusActive)
	c.Assert(created.RegistrationMode, qt.Equals, models.RegistrationModeClosed)
	c.Assert(created.Domain, qt.IsNil)
}

func TestService_CreateTenant_RefusesATakenSlugOrDomain(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc := admin.NewServiceWithFactorySet(memory.NewFactorySet())

	_, err := svc.CreateTenant(ctx, admin.TenantCreateRequest{Name: "Acme", Slug: "acme", Domain: new("inventory.acme.test")})
	c.Assert(err, qt.IsNil)

	_, err = svc.CreateTenant(ctx, admin.TenantCreateRequest{Name: "Other Acme", Slug: "acme"})
	c.Assert(err, qt.ErrorIs, registry.ErrSlugAlreadyExists)

	_, err = svc.CreateTenant(ctx, admin.TenantCreateRequest{Name: "Squatter", Slug: "squatter", Domain: new("inventory.acme.test")})
	c.Assert(err, qt.ErrorIs, admin.ErrTenantDomainTaken)
}

// The name limit counts characters, so a name in a script that takes two
// bytes per character gets the same 100 as an English one (#2919).
func TestService_CreateTenant_NameLimitCountsCharacters(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc := admin.NewServiceWithFactorySet(memory.NewFactorySet())

	_, err := svc.CreateTenant(ctx, admin.TenantCreateRequest{Name: strings.Repeat("é", models.MaxTenantNameRunes), Slug: "fits"})
	c.Assert(err, qt.IsNil)

	_, err = svc.CreateTenant(ctx, admin.TenantCreateRequest{Name: strings.Repeat("é", models.MaxTenantNameRunes+1), Slug: "too-long"})
	verrs, ok := errors.AsType[validation.Errors](err)
	c.Assert(ok, qt.IsTrue, qt.Commentf("err = %v", err))
	c.Assert(verrs["name"], qt.IsNotNil)
}

func TestService_UpdateTenant_SlugAndDomain(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc := admin.NewServiceWithFactorySet(memory.NewFactorySet())

	acme, err := svc.CreateTenant(ctx, admin.TenantCreateRequest{Name: "Acme", Slug: "acme", Domain: new("inventory.acme.test")})
	c.Assert(err, qt.IsNil)
	other, err := svc.CreateTenant(ctx, admin.TenantCreateRequest{Name: "Other", Slug: "other"})
	c.Assert(err, qt.IsNil)

	_, err = svc.UpdateTenant(ctx, other.ID, admin.TenantUpdateRequest{Slug: new("acme")})
	c.Assert(err, qt.ErrorIs, registry.ErrSlugAlreadyExists)

	_, err = svc.UpdateTenant(ctx, other.ID, admin.TenantUpdateRequest{Domain: new("inventory.acme.test")})
	c.Assert(err, qt.ErrorIs, admin.ErrTenantDomainTaken)

	// A tenant keeps its own slug and domain on an unrelated change.
	renamed, err := svc.UpdateTenant(ctx, acme.ID, admin.TenantUpdateRequest{Name: new("Acme Inc"), Slug: new("acme"), Domain: new("inventory.acme.test")})
	c.Assert(err, qt.IsNil)
	c.Assert(renamed.Name, qt.Equals, "Acme Inc")

	// An empty domain removes it rather than storing an empty string.
	cleared, err := svc.UpdateTenant(ctx, acme.ID, admin.TenantUpdateRequest{Domain: new("")})
	c.Assert(err, qt.IsNil)
	c.Assert(cleared.Domain, qt.IsNil)
}

// staleDomainLookup is a tenant registry whose domain lookup finds nothing.
// That is what the lookup of a write sees while another write of the same
// domain has not committed yet, so both pass checkTenantDomainFree and the
// registry is the one to refuse the second.
type staleDomainLookup struct {
	registry.TenantRegistry
}

func (staleDomainLookup) GetByDomain(context.Context, string) (*models.Tenant, error) {
	return nil, registry.ErrNotFound
}

func newRacingDomainService(c *qt.C) *admin.Service {
	c.Helper()
	fs := memory.NewFactorySet()
	fs.TenantRegistry = staleDomainLookup{TenantRegistry: fs.TenantRegistry}
	svc := admin.NewServiceWithFactorySet(fs)
	_, err := svc.CreateTenant(context.Background(), admin.TenantCreateRequest{Name: "Acme", Slug: "acme", Domain: new("inventory.acme.test")})
	c.Assert(err, qt.IsNil)
	return svc
}

// A create that races another one for the same domain gets the 409 the check
// gives, not a 500.
func TestService_CreateTenant_RacingForADomainIsRefusedAsTaken(t *testing.T) {
	c := qt.New(t)
	svc := newRacingDomainService(c)

	_, err := svc.CreateTenant(context.Background(), admin.TenantCreateRequest{Name: "Squatter", Slug: "squatter", Domain: new("inventory.acme.test")})
	c.Assert(err, qt.ErrorIs, admin.ErrTenantDomainTaken)
}

func TestService_UpdateTenant_RacingForADomainIsRefusedAsTaken(t *testing.T) {
	c := qt.New(t)
	svc := newRacingDomainService(c)
	ctx := context.Background()
	other, err := svc.CreateTenant(ctx, admin.TenantCreateRequest{Name: "Other", Slug: "other"})
	c.Assert(err, qt.IsNil)

	_, err = svc.UpdateTenant(ctx, other.ID, admin.TenantUpdateRequest{Domain: new("inventory.acme.test")})
	c.Assert(err, qt.ErrorIs, admin.ErrTenantDomainTaken)
}

// sessionEnv is a tenant with two users and a user in another tenant, each
// holding a refresh token and a personal API token.
type sessionEnv struct {
	fs        *registry.FactorySet
	svc       *admin.Service
	blacklist services.TokenBlacklister
	tenant    *models.Tenant
	members   []*models.User
	outsider  *models.User
}

func newSessionEnv(c *qt.C) *sessionEnv {
	c.Helper()
	ctx := context.Background()
	fs := memory.NewFactorySet()
	env := &sessionEnv{fs: fs, svc: admin.NewServiceWithFactorySet(fs), blacklist: services.NewInMemoryTokenBlacklister()}
	env.svc.SetTokenBlacklist(env.blacklist, time.Hour)

	var err error
	env.tenant, err = env.svc.CreateTenant(ctx, admin.TenantCreateRequest{Name: "Acme", Slug: "acme"})
	c.Assert(err, qt.IsNil)
	other, err := env.svc.CreateTenant(ctx, admin.TenantCreateRequest{Name: "Other", Slug: "other"})
	c.Assert(err, qt.IsNil)

	for _, email := range []string{"ada@acme.test", "bob@acme.test"} {
		env.members = append(env.members, env.seedSignedInUser(c, env.tenant.ID, email))
	}
	env.outsider = env.seedSignedInUser(c, other.ID, "eve@other.test")
	return env
}

func (e *sessionEnv) seedSignedInUser(c *qt.C, tenantID, email string) *models.User {
	c.Helper()
	ctx := context.Background()
	user, err := e.fs.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Email:               email,
		Name:                email,
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	owner := models.TenantUserAwareEntityID{TenantID: tenantID, UserID: user.ID}
	_, err = e.fs.RefreshTokenRegistry.Create(ctx, models.RefreshToken{
		TenantUserAwareEntityID: owner,
		TokenHash:               "refresh-" + user.ID,
		ExpiresAt:               time.Now().Add(24 * time.Hour),
	})
	c.Assert(err, qt.IsNil)
	gen, err := models.GenerateAPIToken()
	c.Assert(err, qt.IsNil)
	_, err = e.fs.APITokenRegistry.Create(ctx, models.APIToken{
		TenantUserAwareEntityID: owner,
		Name:                    "script",
		TokenHash:               "api-" + user.ID,
		TokenPrefix:             gen.DisplayPrefix,
		Scope:                   models.APITokenScopeReadOnly,
	})
	c.Assert(err, qt.IsNil)
	return user
}

// signedIn reports whether the user still has a live refresh token, a live API
// token and no blacklist stamp.
func (e *sessionEnv) signedIn(c *qt.C, user *models.User) bool {
	c.Helper()
	ctx := context.Background()
	refresh, err := e.fs.RefreshTokenRegistry.GetByTokenHash(ctx, "refresh-"+user.ID)
	c.Assert(err, qt.IsNil)
	apiToken, err := e.fs.APITokenRegistry.GetByTokenHash(ctx, "api-"+user.ID)
	c.Assert(err, qt.IsNil)
	_, stamped, err := e.blacklist.UserBlacklistedSince(ctx, user.ID)
	c.Assert(err, qt.IsNil)
	return refresh.IsValid() && !apiToken.IsRevoked() && !stamped
}

func TestService_SuspendTenant_EndsEverySessionInTheTenant(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	env := newSessionEnv(c)

	suspended, err := env.svc.SuspendTenant(ctx, env.tenant.Slug)
	c.Assert(err, qt.IsNil)
	c.Assert(suspended.Status, qt.Equals, models.TenantStatusSuspended)
	for _, member := range env.members {
		c.Assert(env.signedIn(c, member), qt.IsFalse, qt.Commentf("member %s", member.Email))
	}
	c.Assert(env.signedIn(c, env.outsider), qt.IsTrue)

	stored, err := env.fs.TenantRegistry.Get(ctx, env.tenant.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.Status, qt.Equals, models.TenantStatusSuspended)

	// Suspending again is allowed and changes nothing.
	again, err := env.svc.SuspendTenant(ctx, env.tenant.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(again.Status, qt.Equals, models.TenantStatusSuspended)
}

// Reactivating does not bring the sessions back, the same as unblocking a
// user: the users sign in again.
func TestService_ReactivateTenant_LeavesTheSessionsEnded(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	env := newSessionEnv(c)

	_, err := env.svc.SuspendTenant(ctx, env.tenant.ID)
	c.Assert(err, qt.IsNil)
	reactivated, err := env.svc.ReactivateTenant(ctx, env.tenant.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reactivated.Status, qt.Equals, models.TenantStatusActive)
	refresh, err := env.fs.RefreshTokenRegistry.GetByTokenHash(ctx, "refresh-"+env.members[0].ID)
	c.Assert(err, qt.IsNil)
	c.Assert(refresh.IsValid(), qt.IsFalse)
}

// An update that takes the status away from active is a suspension too, so
// `inventario tenants update` with a new status ends the sessions as well.
func TestService_UpdateTenant_StatusChangeEndsSessions(t *testing.T) {
	tests := []struct {
		name     string
		req      admin.TenantUpdateRequest
		signedIn bool
	}{
		{"rename", admin.TenantUpdateRequest{Name: new("Acme Inc")}, true},
		{"status stays active", admin.TenantUpdateRequest{Status: new(models.TenantStatusActive)}, true},
		{"suspended", admin.TenantUpdateRequest{Status: new(models.TenantStatusSuspended)}, false},
		{"inactive", admin.TenantUpdateRequest{Status: new(models.TenantStatusInactive)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			env := newSessionEnv(c)
			_, err := env.svc.UpdateTenant(context.Background(), env.tenant.ID, tt.req)
			c.Assert(err, qt.IsNil)
			c.Assert(env.signedIn(c, env.members[0]), qt.Equals, tt.signedIn)
		})
	}
}

// failingBlacklist refuses every stamp.
type failingBlacklist struct {
	services.TokenBlacklister
}

func (failingBlacklist) BlacklistUserTokens(context.Context, string, time.Duration) error {
	return errors.New("blacklist is down")
}

// A teardown that fails half way leaves the status saved and says so: the
// users are still refused, because their tenant is not active.
func TestService_SuspendTenant_ReportsSessionsItCouldNotEnd(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	env := newSessionEnv(c)
	env.svc.SetTokenBlacklist(failingBlacklist{env.blacklist}, time.Hour)

	suspended, err := env.svc.SuspendTenant(ctx, env.tenant.ID)
	c.Assert(err, qt.ErrorIs, admin.ErrTenantSessionsNotEnded)
	c.Assert(suspended, qt.IsNotNil)
	c.Assert(suspended.Status, qt.Equals, models.TenantStatusSuspended)

	// The other steps still ran for every user.
	for _, member := range env.members {
		refresh, getErr := env.fs.RefreshTokenRegistry.GetByTokenHash(ctx, "refresh-"+member.ID)
		c.Assert(getErr, qt.IsNil)
		c.Assert(refresh.IsValid(), qt.IsFalse)
	}
}

func TestService_SuspendTenant_UnknownTenant(t *testing.T) {
	c := qt.New(t)
	svc := admin.NewServiceWithFactorySet(memory.NewFactorySet())
	_, err := svc.SuspendTenant(context.Background(), "nope")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}
