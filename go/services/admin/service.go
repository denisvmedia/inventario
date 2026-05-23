package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ErrLastSystemAdmin is re-exported from the registry layer for callers
// (CLI, tests) that still import this package. The sentinel itself lives
// at the registry layer so SystemAdminGrantRegistry.RevokeAtomic can
// return it from inside the lock-protected revoke path. New callers
// should branch on registry.ErrLastSystemAdmin directly. #1745 / #1784.
var ErrLastSystemAdmin = registry.ErrLastSystemAdmin

// Service provides administrative operations for CLI commands
type Service struct {
	factorySet *registry.FactorySet
	cleanup    func() error
}

// NewService creates a new admin service with proper registry abstraction
func NewService(dbConfig *shared.DatabaseConfig) (*Service, error) {
	// Validate database configuration
	if err := dbConfig.Validate(); err != nil {
		return nil, fmt.Errorf("database configuration error: %w", err)
	}

	// Check if this is a memory database and reject it for CLI operations
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return nil, fmt.Errorf("CLI operations are not supported for memory databases as they don't provide persistence. Please use a PostgreSQL database")
	}

	// Get registry function using abstraction
	registryFunc, ok := registry.GetRegistry(dbConfig.DBDSN)
	if !ok {
		return nil, fmt.Errorf("unsupported database type in DSN: %s", dbConfig.DBDSN)
	}

	// Create factory set using abstraction
	factorySet, err := registryFunc(registry.Config(dbConfig.DBDSN))
	if err != nil {
		return nil, fmt.Errorf("failed to create registry factory set: %w", err)
	}

	return &Service{
		factorySet: factorySet,
		cleanup:    nil, // Will be set by the registry if needed
	}, nil
}

// Close cleans up resources
func (s *Service) Close() error {
	if s.cleanup != nil {
		return s.cleanup()
	}
	return nil
}

// TenantCreateRequest represents a tenant creation request
type TenantCreateRequest struct {
	Name             string
	Slug             string
	Domain           *string
	Status           models.TenantStatus
	Settings         map[string]any
	Default          bool
	RegistrationMode models.RegistrationMode
}

// TenantUpdateRequest represents a tenant update request
type TenantUpdateRequest struct {
	Name             *string
	Slug             *string
	Domain           *string
	Status           *models.TenantStatus
	Settings         map[string]any
	RegistrationMode *models.RegistrationMode
}

// TenantListRequest represents a tenant list request
type TenantListRequest struct {
	Status string
	Search string
	Limit  int
	Offset int
}

// TenantListResponse represents a tenant list response
type TenantListResponse struct {
	Tenants    []*models.Tenant
	TotalCount int
}

// CreateTenant creates a new tenant
func (s *Service) CreateTenant(ctx context.Context, req TenantCreateRequest) (*models.Tenant, error) {
	mode := req.RegistrationMode
	if mode == "" {
		mode = models.RegistrationModeClosed
	}
	tenant := &models.Tenant{
		Name:             req.Name,
		Slug:             req.Slug,
		Domain:           req.Domain,
		Status:           req.Status,
		IsDefault:        req.Default,
		Settings:         req.Settings,
		RegistrationMode: mode,
	}

	// Validate tenant
	if err := tenant.ValidateWithContext(ctx); err != nil {
		return nil, fmt.Errorf("tenant validation failed: %w", err)
	}

	// Create tenant using factory set
	createdTenant, err := s.factorySet.TenantRegistry.Create(ctx, *tenant)
	if err != nil {
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}

	return createdTenant, nil
}

// GetTenant retrieves a tenant by ID or slug
func (s *Service) GetTenant(ctx context.Context, idOrSlug string) (*models.Tenant, error) {
	// Try by ID first
	tenant, err := s.factorySet.TenantRegistry.Get(ctx, idOrSlug)
	if err != nil {
		// Try by slug
		tenant, err = s.factorySet.TenantRegistry.GetBySlug(ctx, idOrSlug)
		if err != nil {
			return nil, fmt.Errorf("tenant '%s' not found (tried both ID and slug)", idOrSlug)
		}
	}

	return tenant, nil
}

// ListTenants lists tenants with filtering and pagination
func (s *Service) ListTenants(ctx context.Context, req TenantListRequest) (*TenantListResponse, error) {
	// Get all tenants first
	allTenants, err := s.factorySet.TenantRegistry.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tenants: %w", err)
	}

	// Apply filters in memory (could be optimized with database-level filtering)
	var filteredTenants []*models.Tenant
	for _, tenant := range allTenants {
		if s.matchesTenantFilters(tenant, req) {
			filteredTenants = append(filteredTenants, tenant)
		}
	}

	// Apply pagination
	totalCount := len(filteredTenants)
	start := min(req.Offset, len(filteredTenants))

	end := min(start+req.Limit, len(filteredTenants))

	paginatedTenants := filteredTenants[start:end]

	return &TenantListResponse{
		Tenants:    paginatedTenants,
		TotalCount: totalCount,
	}, nil
}

// UpdateTenant updates a tenant
func (s *Service) UpdateTenant(ctx context.Context, idOrSlug string, req TenantUpdateRequest) (*models.Tenant, error) {
	// Get existing tenant
	tenant, err := s.GetTenant(ctx, idOrSlug)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if req.Name != nil {
		tenant.Name = *req.Name
	}
	if req.Slug != nil {
		tenant.Slug = *req.Slug
	}
	if req.Domain != nil {
		tenant.Domain = req.Domain
	}
	if req.Status != nil {
		tenant.Status = *req.Status
	}
	if req.Settings != nil {
		tenant.Settings = req.Settings
	}
	if req.RegistrationMode != nil {
		tenant.RegistrationMode = *req.RegistrationMode
	}

	// Validate updated tenant
	if err := tenant.ValidateWithContext(ctx); err != nil {
		return nil, fmt.Errorf("tenant validation failed: %w", err)
	}

	// Update tenant
	updatedTenant, err := s.factorySet.TenantRegistry.Update(ctx, *tenant)
	if err != nil {
		return nil, fmt.Errorf("failed to update tenant: %w", err)
	}

	return updatedTenant, nil
}

// DeleteTenant deletes a tenant
func (s *Service) DeleteTenant(ctx context.Context, idOrSlug string) error {
	// Get existing tenant to validate it exists
	tenant, err := s.GetTenant(ctx, idOrSlug)
	if err != nil {
		return err
	}

	// Delete tenant
	if err := s.factorySet.TenantRegistry.Delete(ctx, tenant.ID); err != nil {
		return fmt.Errorf("failed to delete tenant: %w", err)
	}

	return nil
}

// GetTenantUserCount returns the number of users in a tenant
func (s *Service) GetTenantUserCount(ctx context.Context, tenantID string) (int, error) {
	// Use the ListByTenant method which is available on UserRegistry
	users, err := s.factorySet.UserRegistry.ListByTenant(ctx, tenantID)
	if err != nil {
		return 0, fmt.Errorf("failed to list users in tenant: %w", err)
	}

	return len(users), nil
}

// UserCreateRequest represents a user creation request
type UserCreateRequest struct {
	Email    string
	Password string
	Name     string
	TenantID string
	IsActive bool
}

// UserUpdateRequest represents a user update request
type UserUpdateRequest struct {
	Email    *string
	Name     *string
	IsActive *bool
	TenantID *string
	Password *string
}

// UserListRequest represents a user list request
type UserListRequest struct {
	TenantID string
	Active   *bool
	Search   string
	Limit    int
	Offset   int
}

// UserListResponse represents a user list response
type UserListResponse struct {
	Users      []*models.User
	TotalCount int
}

// CreateUser creates a new user
func (s *Service) CreateUser(ctx context.Context, req UserCreateRequest) (*models.User, error) {
	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: req.TenantID,
		},
		Email:    req.Email,
		Name:     req.Name,
		IsActive: req.IsActive,
	}

	// Set password
	if err := user.SetPassword(req.Password); err != nil {
		return nil, fmt.Errorf("failed to set password: %w", err)
	}

	// Validate user
	if err := user.ValidateWithContext(ctx); err != nil {
		return nil, fmt.Errorf("user validation failed: %w", err)
	}

	// Create user
	createdUser, err := s.factorySet.UserRegistry.Create(ctx, *user)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return createdUser, nil
}

// GetUser retrieves a user by ID or email
func (s *Service) GetUser(ctx context.Context, idOrEmail string) (*models.User, error) {
	// Try by ID first
	user, err := s.factorySet.UserRegistry.Get(ctx, idOrEmail)
	if err != nil {
		// The UserRegistry.GetByEmail requires tenantID, so we need to search through all users
		// This is not ideal but works with the current registry interface
		allUsers, listErr := s.factorySet.UserRegistry.List(ctx)
		if listErr != nil {
			return nil, fmt.Errorf("user '%s' not found (tried ID, failed to search by email: %w)", idOrEmail, listErr)
		}

		// Search for user by email
		for _, u := range allUsers {
			if u.Email == idOrEmail {
				return u, nil
			}
		}

		return nil, fmt.Errorf("user '%s' not found (tried both ID and email)", idOrEmail)
	}

	return user, nil
}

// ListUsers lists users with filtering and pagination
func (s *Service) ListUsers(ctx context.Context, req UserListRequest) (*UserListResponse, error) {
	var allUsers []*models.User
	var err error

	// If tenant filter is specified, use the optimized ListByTenant method
	if req.TenantID != "" {
		allUsers, err = s.factorySet.UserRegistry.ListByTenant(ctx, req.TenantID)
		if err != nil {
			return nil, fmt.Errorf("failed to list users by tenant: %w", err)
		}
	} else {
		// Get all users
		allUsers, err = s.factorySet.UserRegistry.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list users: %w", err)
		}
	}

	// Apply additional filters in memory
	var filteredUsers []*models.User
	for _, user := range allUsers {
		if s.matchesUserFilters(user, req) {
			filteredUsers = append(filteredUsers, user)
		}
	}

	// Apply pagination
	totalCount := len(filteredUsers)
	start := min(req.Offset, len(filteredUsers))

	end := min(start+req.Limit, len(filteredUsers))

	paginatedUsers := filteredUsers[start:end]

	return &UserListResponse{
		Users:      paginatedUsers,
		TotalCount: totalCount,
	}, nil
}

// UpdateUser updates a user
func (s *Service) UpdateUser(ctx context.Context, idOrEmail string, req UserUpdateRequest) (*models.User, error) {
	// Get existing user
	user, err := s.GetUser(ctx, idOrEmail)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}
	if req.TenantID != nil {
		user.SetTenantID(*req.TenantID)
	}
	if req.Password != nil {
		if err := user.SetPassword(*req.Password); err != nil {
			return nil, fmt.Errorf("failed to set password: %w", err)
		}
	}

	// Validate updated user
	if err := user.ValidateWithContext(ctx); err != nil {
		return nil, fmt.Errorf("user validation failed: %w", err)
	}

	// Update user
	updatedUser, err := s.factorySet.UserRegistry.Update(ctx, *user)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return updatedUser, nil
}

// DeleteUser deletes a user
func (s *Service) DeleteUser(ctx context.Context, idOrEmail string) error {
	// Get existing user to validate it exists
	user, err := s.GetUser(ctx, idOrEmail)
	if err != nil {
		return err
	}

	// Delete user
	if err := s.factorySet.UserRegistry.Delete(ctx, user.ID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// ResetUserMFA removes the user's `user_mfa_secrets` row (if any) and
// emits a `login_events` row with outcome=mfa_admin_reset so the user
// later sees "an administrator removed your second factor" in their
// login history. Idempotent — calling on a user without MFA enrolled
// returns nil with a flag indicating no row was touched.
//
// The recovery story per #1380 v1 is "contact support"; this is the
// support-side action. The user can re-enroll afterwards through the
// normal Settings → Privacy & Security flow.
func (s *Service) ResetUserMFA(ctx context.Context, idOrEmail string) (resetUser *models.User, hadEnrollment bool, err error) {
	user, err := s.GetUser(ctx, idOrEmail)
	if err != nil {
		return nil, false, err
	}

	if s.factorySet.UserMFASecretRegistry == nil {
		return user, false, fmt.Errorf("MFA registry not configured")
	}

	_, lookupErr := s.factorySet.UserMFASecretRegistry.GetByUser(ctx, user.TenantID, user.ID)
	switch {
	case lookupErr == nil:
		hadEnrollment = true
	case errors.Is(lookupErr, registry.ErrNotFound):
		// Idempotent — no row to delete.
	default:
		return user, false, fmt.Errorf("failed to look up MFA enrollment: %w", lookupErr)
	}

	if err := s.factorySet.UserMFASecretRegistry.DeleteByUser(ctx, user.TenantID, user.ID); err != nil {
		return user, hadEnrollment, fmt.Errorf("failed to delete MFA enrollment: %w", err)
	}

	// Append-only login_events row so the user sees the admin reset
	// next time they look at their login history. UserAgent + IPAddress
	// are left blank because the actor is an operator on the CLI, not
	// an HTTP request.
	if s.factorySet.LoginEventRegistry != nil {
		userID := user.ID
		event := models.LoginEvent{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
			UserID:              &userID,
			Email:               user.Email,
			Outcome:             models.LoginOutcomeMFAAdminReset,
			Method:              models.LoginMethodPassword,
		}
		if _, evErr := s.factorySet.LoginEventRegistry.Create(ctx, event); evErr != nil {
			// Best-effort — the row delete already succeeded; we
			// don't want to fail the whole operation because the
			// audit write blipped.
			return user, hadEnrollment, fmt.Errorf("mfa reset succeeded but login_events write failed: %w", evErr)
		}
	}

	return user, hadEnrollment, nil
}

// GrantSystemAdmin records a row in `system_admin_grants` (#1784) for
// the user resolved by idOrEmail. Idempotent — calling on an already-
// admin user returns the row unchanged with hadFlag=true so the CLI can
// print "already a system admin" rather than a misleading "granted"
// line. Writes a `admin.grant_system_admin` audit row regardless so the
// audit trail shows the attempt.
//
// The CLI runs out-of-band with no authenticated operator, so the
// grant row's granted_by column is nil — the audit row carries the
// "operator was an OS/host boundary" signal via UserID=nil (see
// logAdminAction).
func (s *Service) GrantSystemAdmin(ctx context.Context, idOrEmail string) (resultUser *models.User, hadFlag bool, err error) {
	user, err := s.GetUser(ctx, idOrEmail)
	if err != nil {
		s.logAdminAction(ctx, "admin.grant_system_admin", nil, "", err)
		return nil, false, err
	}

	if s.factorySet.SystemAdminGrantRegistry == nil {
		configErr := errxtrace.Classify(registry.ErrInvalidConfig, errx.Attrs("missing", "SystemAdminGrantRegistry"))
		// Audit the misconfiguration before returning so the trail records
		// the attempt even when the grant store is unwired. The subject was
		// resolved, so charge the attempt to that tenant/user.
		s.logAdminAction(ctx, "admin.grant_system_admin", &user.TenantID, user.ID, configErr)
		return nil, false, configErr
	}

	hadFlag, grantErr := s.factorySet.SystemAdminGrantRegistry.Grant(ctx, user.ID, nil)
	if grantErr != nil {
		s.logAdminAction(ctx, "admin.grant_system_admin", &user.TenantID, user.ID, grantErr)
		return nil, false, errxtrace.Wrap("failed to grant system-admin", grantErr)
	}

	s.logAdminAction(ctx, "admin.grant_system_admin", &user.TenantID, user.ID, nil)
	return user, hadFlag, nil
}

// RevokeSystemAdmin deletes the resolved user's row from
// `system_admin_grants` (#1784). When allowZero is false (the default),
// the registry refuses to revoke the last remaining system admin so an
// operator can't lock themselves out of every admin surface — the
// count is checked AND the row is deleted inside the same transaction
// (postgres) / under the same registry mutex (memory), so the operation
// is atomic against concurrent revokes. Idempotent: revoking a user who
// holds no grant returns hadFlag=false with no error.
//
// allowZero=true bypasses the guard; intended for the deliberate
// "I'm shutting down the platform" path, exposed on the CLI as
// `--allow-zero`.
//
// safety-override toggle, not control coupling; the alternative would
// be a sibling RevokeSystemAdminAllowZero method which adds API
// surface without changing the behaviour story.
//
//revive:disable-next-line:flag-parameter — allowZero is a deliberate
func (s *Service) RevokeSystemAdmin(ctx context.Context, idOrEmail string, allowZero bool) (resultUser *models.User, hadFlag bool, err error) {
	user, err := s.GetUser(ctx, idOrEmail)
	if err != nil {
		s.logAdminAction(ctx, "admin.revoke_system_admin", nil, "", err)
		return nil, false, err
	}

	if s.factorySet.SystemAdminGrantRegistry == nil {
		configErr := errxtrace.Classify(registry.ErrInvalidConfig, errx.Attrs("missing", "SystemAdminGrantRegistry"))
		// Mirror GrantSystemAdmin: audit the misconfiguration before
		// returning so the trail records the attempt regardless. Tenant +
		// acting user are nil/"" because this is a server-config failure.
		s.logAdminAction(ctx, "admin.revoke_system_admin", nil, "", configErr)
		return nil, false, configErr
	}

	hadFlag, revokeErr := s.factorySet.SystemAdminGrantRegistry.RevokeAtomic(ctx, user.ID, allowZero)
	if revokeErr != nil {
		s.logAdminAction(ctx, "admin.revoke_system_admin", &user.TenantID, user.ID, revokeErr)
		if errors.Is(revokeErr, registry.ErrLastSystemAdmin) {
			// Return the sentinel unwrapped so callers branching on
			// errors.Is continue to work; also lets the CLI render the
			// friendly --allow-zero hint without consuming the wrap.
			return nil, hadFlag, revokeErr
		}
		return nil, hadFlag, errxtrace.Wrap("failed to revoke system-admin", revokeErr)
	}

	s.logAdminAction(ctx, "admin.revoke_system_admin", &user.TenantID, user.ID, nil)
	return user, hadFlag, nil
}

// SystemAdminListing is the joined view a CLI render needs to show
// each grant row: identity fields from `users` plus the real
// `granted_at` timestamp from `system_admin_grants` (no longer the
// `users.updated_at` proxy that the pre-#1784 path used).
type SystemAdminListing struct {
	User      *models.User
	GrantedAt time.Time
	GrantedBy *string
}

// ListSystemAdmins returns every system-admin grant joined to its
// user row, ordered by granted_at ASC. Logs an
// `admin.list_system_admins` audit row regardless of result count so
// the trail shows operator-side reads as well as writes.
func (s *Service) ListSystemAdmins(ctx context.Context) ([]*SystemAdminListing, error) {
	if s.factorySet.SystemAdminGrantRegistry == nil {
		err := errxtrace.Classify(registry.ErrInvalidConfig, errx.Attrs("missing", "SystemAdminGrantRegistry"))
		s.logAdminAction(ctx, "admin.list_system_admins", nil, "", err)
		return nil, err
	}

	grants, err := s.factorySet.SystemAdminGrantRegistry.List(ctx)
	if err != nil {
		s.logAdminAction(ctx, "admin.list_system_admins", nil, "", err)
		return nil, errxtrace.Wrap("failed to list system admin grants", err)
	}

	out := make([]*SystemAdminListing, 0, len(grants))
	for _, g := range grants {
		// Per-grant Get keeps the registry interface narrow; for the
		// expected single-digit grant count this is cheaper than
		// introducing a join helper. A user that disappeared between
		// the List and the Get (ON DELETE CASCADE on the FK) is
		// skipped silently — the CLI then renders a shorter list,
		// which matches the post-cascade truth.
		user, getErr := s.factorySet.UserRegistry.Get(ctx, g.UserID)
		if getErr != nil {
			if errors.Is(getErr, registry.ErrNotFound) {
				continue
			}
			s.logAdminAction(ctx, "admin.list_system_admins", nil, "", getErr)
			return nil, errxtrace.Wrap("failed to fetch grant subject user", getErr)
		}
		out = append(out, &SystemAdminListing{
			User:      user,
			GrantedAt: g.GrantedAt,
			GrantedBy: g.GrantedBy,
		})
	}

	s.logAdminAction(ctx, "admin.list_system_admins", nil, "", nil)
	return out, nil
}

// logAdminAction writes an admin audit row via the AuditLogRegistry on
// the factory set. Best-effort: write failures are tolerated because the
// CLI surfaces them via slog; the operator-visible result is still the
// CLI's own return value. We don't go through services.AuditService here
// because the CLI is not built around a *services.AuditService — it
// holds the factory set directly. Mirrors the row shape that
// AuditService.LogAdmin would produce (action / user_id / entity_type /
// entity_id / success / error_message), but without the HTTP fields:
// CLI invocations have no IP / User-Agent and no impersonation context.
//
// Actor convention: the CLI runs out-of-band with no JWT-authenticated
// operator, so UserID (the actor) is intentionally nil for every CLI
// invocation — "operator" identity for CLI runs is the OS/host
// boundary, not a row in this database. The subject of the action is
// stored as EntityID. When the HTTP admin path lands it will populate
// UserID from the impersonation JWT's actor claim (#1744 umbrella).
func (s *Service) logAdminAction(ctx context.Context, action string, tenantID *string, subjectUserID string, opErr error) {
	if s.factorySet == nil || s.factorySet.AuditLogRegistry == nil {
		return
	}

	entry := models.AuditLog{
		Action:   action,
		UserID:   nil, // CLI invocations have no authenticated actor — see method doc.
		TenantID: tenantID,
		Success:  opErr == nil,
	}
	if subjectUserID != "" {
		subjectType := "user"
		entry.EntityType = &subjectType
		entry.EntityID = &subjectUserID
	}
	if opErr != nil {
		msg := opErr.Error()
		entry.ErrorMessage = &msg
	}

	if _, createErr := s.factorySet.AuditLogRegistry.Create(ctx, entry); createErr != nil {
		// Best-effort: the CLI surfaces success/failure of the operation
		// itself; a missing audit row is recoverable. Log via slog so
		// operators with audit-completeness monitoring can still notice.
		slog.Error("Failed to write admin audit log entry",
			"action", action, "error", createErr)
	}
}

// matchesTenantFilters checks if a tenant matches the given filters
func (s *Service) matchesTenantFilters(tenant *models.Tenant, req TenantListRequest) bool {
	// Status filter
	if req.Status != "" && string(tenant.Status) != req.Status {
		return false
	}

	// Search filter (name or slug)
	if req.Search != "" {
		searchLower := strings.ToLower(req.Search)
		if !strings.Contains(strings.ToLower(tenant.Name), searchLower) &&
			!strings.Contains(strings.ToLower(tenant.Slug), searchLower) {
			return false
		}
	}

	return true
}

// matchesUserFilters checks if a user matches the given filters
func (s *Service) matchesUserFilters(user *models.User, req UserListRequest) bool {
	// Active filter
	if req.Active != nil && user.IsActive != *req.Active {
		return false
	}

	// Search filter (email or name)
	if req.Search != "" {
		searchLower := strings.ToLower(req.Search)
		if !strings.Contains(strings.ToLower(user.Email), searchLower) &&
			!strings.Contains(strings.ToLower(user.Name), searchLower) {
			return false
		}
	}

	return true
}
