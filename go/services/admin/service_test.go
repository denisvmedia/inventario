package admin

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

func TestService_RevokeSystemAdmin_MissingGrantRegistryAuditsResolvedSubject(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	factorySet := memory.NewFactorySet()
	user, err := factorySet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
		Email:               "subject@example.com",
		Name:                "Subject User",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)

	svc := &Service{factorySet: factorySet}
	svc.factorySet.SystemAdminGrantRegistry = nil

	resultUser, hadFlag, err := svc.RevokeSystemAdmin(ctx, user.ID, false)
	c.Assert(resultUser, qt.IsNil)
	c.Assert(hadFlag, qt.IsFalse)
	c.Assert(err, qt.ErrorIs, registry.ErrInvalidConfig)

	entries, err := factorySet.AuditLogRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(entries, qt.HasLen, 1)

	entry := entries[0]
	c.Assert(entry.Action, qt.Equals, "admin.revoke_system_admin")
	c.Assert(entry.Success, qt.IsFalse)
	c.Assert(entry.UserID, qt.IsNil)
	c.Assert(entry.TenantID, qt.IsNotNil)
	c.Assert(*entry.TenantID, qt.Equals, user.TenantID)
	c.Assert(entry.EntityType, qt.IsNotNil)
	c.Assert(*entry.EntityType, qt.Equals, "user")
	c.Assert(entry.EntityID, qt.IsNotNil)
	c.Assert(*entry.EntityID, qt.Equals, user.ID)
	c.Assert(entry.ErrorMessage, qt.IsNotNil)
}

// seedUser creates a user in the memory factory set and returns it. The admin
// service resolves subjects by id or email, so both are set.
func seedUser(c *qt.C, fs *registry.FactorySet, email string) *models.User {
	c.Helper()
	user, err := fs.UserRegistry.Create(context.Background(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
		Email:               email,
		Name:                email,
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	return user
}

func TestService_GrantSystemAdmin_IsIdempotentAndAudited(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}
	user := seedUser(c, fs, "grantee@example.com")

	granted, hadFlag, err := svc.GrantSystemAdmin(ctx, user.Email)
	c.Assert(err, qt.IsNil)
	c.Assert(granted.ID, qt.Equals, user.ID)
	// hadFlag reports the PRIOR state, which is what lets the CLI say
	// "already an admin" instead of claiming it changed something.
	c.Assert(hadFlag, qt.IsFalse)

	_, hadFlag, err = svc.GrantSystemAdmin(ctx, user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(hadFlag, qt.IsTrue)

	exists, err := fs.SystemAdminGrantRegistry.Exists(ctx, user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsTrue)

	entries, err := fs.AuditLogRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(entries, qt.HasLen, 2)
	for _, entry := range entries {
		c.Assert(entry.Action, qt.Equals, "admin.grant_system_admin")
		c.Assert(entry.Success, qt.IsTrue)
	}
}

func TestService_GrantSystemAdmin_UnknownSubjectIsAudited(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}

	_, _, err := svc.GrantSystemAdmin(ctx, "nobody@example.com")
	c.Assert(err, qt.IsNotNil)

	// A failed attempt is the one an operator most wants in the trail.
	entries, err := fs.AuditLogRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(entries, qt.HasLen, 1)
	c.Assert(entries[0].Action, qt.Equals, "admin.grant_system_admin")
	c.Assert(entries[0].Success, qt.IsFalse)
}

// The guard that stops an operator locking every admin surface against
// themselves. The count check and the delete are one atomic step in the
// registry; this covers the service's half of it.
func TestService_RevokeSystemAdmin_RefusesTheLastAdmin(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}
	only := seedUser(c, fs, "only-admin@example.com")

	_, _, err := svc.GrantSystemAdmin(ctx, only.ID)
	c.Assert(err, qt.IsNil)

	_, _, err = svc.RevokeSystemAdmin(ctx, only.ID, false)
	// The sentinel must come back unwrapped: the CLI branches on it to print
	// the --allow-zero hint.
	c.Assert(err, qt.ErrorIs, registry.ErrLastSystemAdmin)

	exists, err := fs.SystemAdminGrantRegistry.Exists(ctx, only.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsTrue, qt.Commentf("the refused revoke must not have deleted the grant"))
}

func TestService_RevokeSystemAdmin_AllowZeroBypassesTheGuard(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}
	only := seedUser(c, fs, "last-admin@example.com")

	_, _, err := svc.GrantSystemAdmin(ctx, only.ID)
	c.Assert(err, qt.IsNil)

	revoked, hadFlag, err := svc.RevokeSystemAdmin(ctx, only.ID, true)
	c.Assert(err, qt.IsNil)
	c.Assert(revoked.ID, qt.Equals, only.ID)
	c.Assert(hadFlag, qt.IsTrue)

	exists, err := fs.SystemAdminGrantRegistry.Exists(ctx, only.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse)
}

func TestService_RevokeSystemAdmin_NonAdminIsANoOp(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}
	keeper := seedUser(c, fs, "keeper@example.com")
	plain := seedUser(c, fs, "plain@example.com")

	// Another admin exists, so the last-admin guard is not what answers here.
	_, _, err := svc.GrantSystemAdmin(ctx, keeper.ID)
	c.Assert(err, qt.IsNil)

	_, hadFlag, err := svc.RevokeSystemAdmin(ctx, plain.ID, false)
	c.Assert(err, qt.IsNil)
	c.Assert(hadFlag, qt.IsFalse)
}

func TestService_ListSystemAdmins_JoinsTheGrantToItsUser(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}
	admin := seedUser(c, fs, "listed@example.com")

	_, _, err := svc.GrantSystemAdmin(ctx, admin.ID)
	c.Assert(err, qt.IsNil)

	listing, err := svc.ListSystemAdmins(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(listing, qt.HasLen, 1)
	c.Assert(listing[0].User.Email, qt.Equals, admin.Email)
	// granted_at comes from the grant row, not from users.updated_at.
	c.Assert(listing[0].GrantedAt.IsZero(), qt.IsFalse)
}

func TestService_DeleteUser_PurgesDependentsFirst(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}
	user := seedUser(c, fs, "deleteme@example.com")

	// A dependent row the purge has to take with it: the users row cannot be
	// dropped while an auth child still points at it.
	_, err := fs.SystemAdminGrantRegistry.Grant(ctx, user.ID, nil)
	c.Assert(err, qt.IsNil)

	c.Assert(svc.DeleteUser(ctx, user.Email), qt.IsNil)

	_, err = fs.UserRegistry.Get(ctx, user.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	exists, err := fs.SystemAdminGrantRegistry.Exists(ctx, user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse, qt.Commentf("the grant must go with the user, not outlive them"))
}

func TestService_DeleteUser_UnknownSubject(t *testing.T) {
	c := qt.New(t)
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}

	err := svc.DeleteUser(context.Background(), "ghost@example.com")
	c.Assert(err, qt.IsNotNil)
}

func TestService_ResetUserMFA_IsIdempotent(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}
	user := seedUser(c, fs, "mfa-reset@example.com")

	// Not enrolled: the support action must succeed and say nothing was
	// touched, rather than failing at the operator.
	resetUser, hadEnrollment, err := svc.ResetUserMFA(ctx, user.Email)
	c.Assert(err, qt.IsNil)
	c.Assert(resetUser.ID, qt.Equals, user.ID)
	c.Assert(hadEnrollment, qt.IsFalse)

	_, err = fs.UserMFASecretRegistry.Create(ctx, models.UserMFASecret{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: user.TenantID,
			UserID:   user.ID,
		},
		SecretEncrypted: "cipher",
	})
	c.Assert(err, qt.IsNil)

	_, hadEnrollment, err = svc.ResetUserMFA(ctx, user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(hadEnrollment, qt.IsTrue)

	_, err = fs.UserMFASecretRegistry.GetByUser(ctx, user.TenantID, user.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestService_TenantLifecycle(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}

	created, err := svc.CreateTenant(ctx, TenantCreateRequest{
		Name:   "Acme",
		Slug:   "acme",
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)
	// An unset registration mode must not leave the tenant open to the
	// world; closed is the safe default and the one the API relies on.
	c.Assert(created.RegistrationMode, qt.Equals, models.RegistrationModeClosed)

	// Both handles resolve to the same row — the CLI takes either.
	bySlug, err := svc.GetTenant(ctx, "acme")
	c.Assert(err, qt.IsNil)
	c.Assert(bySlug.ID, qt.Equals, created.ID)
	byID, err := svc.GetTenant(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(byID.Slug, qt.Equals, "acme")

	name := "Acme Inc"
	mode := models.RegistrationModeOpen
	updated, err := svc.UpdateTenant(ctx, "acme", TenantUpdateRequest{
		Name:             &name,
		RegistrationMode: &mode,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(updated.Name, qt.Equals, "Acme Inc")
	c.Assert(updated.RegistrationMode, qt.Equals, models.RegistrationModeOpen)
	// A nil field is "leave it alone", not "clear it".
	c.Assert(updated.Slug, qt.Equals, "acme")

	_, err = svc.GetTenant(ctx, "nope")
	c.Assert(err, qt.IsNotNil)
}

func TestService_DeleteTenant_PurgesDependentsFirst(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}

	tenant, err := svc.CreateTenant(ctx, TenantCreateRequest{
		Name:   "Doomed",
		Slug:   "doomed",
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	user, err := svc.CreateUser(ctx, UserCreateRequest{
		Email:    "resident@example.com",
		Password: "Password123",
		Name:     "Resident",
		TenantID: tenant.ID,
		IsActive: true,
	})
	c.Assert(err, qt.IsNil)

	count, err := svc.GetTenantUserCount(ctx, tenant.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)

	c.Assert(svc.DeleteTenant(ctx, "doomed"), qt.IsNil)

	_, err = fs.TenantRegistry.Get(ctx, tenant.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	// The tenant row can only go once its children have: a surviving user
	// would be a row pointing at a tenant that no longer exists.
	_, err = fs.UserRegistry.Get(ctx, user.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestService_UserLifecycleAndFilters(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}

	tenant, err := svc.CreateTenant(ctx, TenantCreateRequest{
		Name: "Filters", Slug: "filters", Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	active, err := svc.CreateUser(ctx, UserCreateRequest{
		Email: "active@example.com", Password: "Password123",
		Name: "Active One", TenantID: tenant.ID, IsActive: true,
	})
	c.Assert(err, qt.IsNil)
	// The password must not survive as plaintext anywhere on the row.
	c.Assert(active.PasswordHash, qt.Not(qt.Equals), "Password123")
	c.Assert(active.CheckPassword("Password123"), qt.IsTrue)

	_, err = svc.CreateUser(ctx, UserCreateRequest{
		Email: "dormant@example.com", Password: "Password123",
		Name: "Dormant One", TenantID: tenant.ID, IsActive: false,
	})
	c.Assert(err, qt.IsNil)

	all, err := svc.ListUsers(ctx, UserListRequest{TenantID: tenant.ID})
	c.Assert(err, qt.IsNil)
	c.Assert(all.Users, qt.HasLen, 2)
	c.Assert(all.TotalCount, qt.Equals, 2)

	onlyActive := true
	filtered, err := svc.ListUsers(ctx, UserListRequest{TenantID: tenant.ID, Active: &onlyActive})
	c.Assert(err, qt.IsNil)
	c.Assert(filtered.Users, qt.HasLen, 1)
	c.Assert(filtered.Users[0].Email, qt.Equals, "active@example.com")

	searched, err := svc.ListUsers(ctx, UserListRequest{TenantID: tenant.ID, Search: "dormant"})
	c.Assert(err, qt.IsNil)
	c.Assert(searched.Users, qt.HasLen, 1)
	c.Assert(searched.Users[0].Email, qt.Equals, "dormant@example.com")

	// TotalCount is the size of the filtered set, not of the page, or a UI
	// paginator would promise pages that do not exist.
	page, err := svc.ListUsers(ctx, UserListRequest{TenantID: tenant.ID, Limit: 1})
	c.Assert(err, qt.IsNil)
	c.Assert(page.Users, qt.HasLen, 1)
	c.Assert(page.TotalCount, qt.Equals, 2)

	deactivated := false
	updated, err := svc.UpdateUser(ctx, "active@example.com", UserUpdateRequest{IsActive: &deactivated})
	c.Assert(err, qt.IsNil)
	c.Assert(updated.IsActive, qt.IsFalse)
	c.Assert(updated.Email, qt.Equals, "active@example.com")
}

func TestService_ListTenants_FiltersAndPages(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}

	for _, spec := range []struct {
		name, slug string
		status     models.TenantStatus
	}{
		{"Alpha", "alpha", models.TenantStatusActive},
		{"Beta", "beta", models.TenantStatusSuspended},
		{"Gamma", "gamma", models.TenantStatusActive},
	} {
		_, err := svc.CreateTenant(ctx, TenantCreateRequest{
			Name: spec.name, Slug: spec.slug, Status: spec.status,
		})
		c.Assert(err, qt.IsNil)
	}

	all, err := svc.ListTenants(ctx, TenantListRequest{})
	c.Assert(err, qt.IsNil)
	c.Assert(all.Tenants, qt.HasLen, 3)

	activeOnly, err := svc.ListTenants(ctx, TenantListRequest{Status: string(models.TenantStatusActive)})
	c.Assert(err, qt.IsNil)
	c.Assert(activeOnly.Tenants, qt.HasLen, 2)

	searched, err := svc.ListTenants(ctx, TenantListRequest{Search: "bet"})
	c.Assert(err, qt.IsNil)
	c.Assert(searched.Tenants, qt.HasLen, 1)
	c.Assert(searched.Tenants[0].Slug, qt.Equals, "beta")

	offset, err := svc.ListTenants(ctx, TenantListRequest{Limit: 2, Offset: 2})
	c.Assert(err, qt.IsNil)
	c.Assert(offset.Tenants, qt.HasLen, 1)
	c.Assert(offset.TotalCount, qt.Equals, 3)
}
