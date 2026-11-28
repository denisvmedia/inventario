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

	"go.5x5.cz/inventario/cmd/inventario/shared"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
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
	// fileService is optional. When set (via SetFileService), DeleteTenant
	// deletes the tenant's physical blobs before purging its DB rows. When
	// nil, blob cleanup is skipped with a loud WARNING — the DB rows are
	// still purged, but object-storage objects are orphaned (the caller must
	// wire a fileService to get full cleanup). It is a setter rather than a
	// constructor parameter so the 20-odd existing NewService(dbConfig)
	// callers that never delete a tenant don't have to thread an upload
	// location they don't need.
	fileService *services.FileService
	// avatars is optional. When set (via SetAvatarService), a tenant move
	// copies the user's profile photo into the new tenant's storage, and
	// DeleteUser deletes the photo of the user it deletes. A move of a user
	// who has a photo is refused while it is nil; a delete leaves the photo.
	avatars *services.AvatarService
	// blacklist is optional. When set (via SetTokenBlacklist), ending a
	// tenant's sessions stamps it for every user of the tenant, for
	// blacklistTTL.
	blacklist    services.TokenBlacklister
	blacklistTTL time.Duration
}

// NewServiceWithFactorySet returns a service over an already opened factory
// set. NewService is the constructor the CLI uses; this one serves callers
// that hold a FactorySet, such as tests against the memory backend.
func NewServiceWithFactorySet(factorySet *registry.FactorySet) *Service {
	return &Service{factorySet: factorySet}
}

// SetAvatarService wires the avatar service a tenant move copies the user's
// profile photo with and DeleteUser deletes it with. Callers that may move or
// delete users (the users update and delete CLIs, which build it from the
// --upload-location flag) must call this.
func (s *Service) SetAvatarService(avatars *services.AvatarService) {
	s.avatars = avatars
}

// SetFileService wires the optional file service used by DeleteTenant to
// remove a tenant's physical blobs before its DB rows are purged. Callers that
// perform a tenant hard-delete must call this (e.g. the tenants delete CLI,
// which builds it from the --upload-location flag); other admin callers can
// leave it unset.
func (s *Service) SetFileService(fileService *services.FileService) {
	s.fileService = fileService
}

// FactorySet exposes the underlying registry factory set so callers (e.g. the
// tenants delete CLI) can construct a matching FileService bound to the same
// backend before invoking DeleteTenant.
func (s *Service) FactorySet() *registry.FactorySet {
	return s.factorySet
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
		// Not the DSN itself: it carries the password, and this error is
		// printed by the command.
		return nil, errors.New("unsupported database type in DSN (expected postgres://)")
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

// CreateTenant creates a new tenant. An empty Slug is generated from the
// name (GenerateTenantSlug), an empty Status means active, and an empty
// RegistrationMode means closed. A slug or domain another tenant already has
// is refused with registry.ErrSlugAlreadyExists or ErrTenantDomainTaken.
func (s *Service) CreateTenant(ctx context.Context, req TenantCreateRequest) (*models.Tenant, error) {
	mode := req.RegistrationMode
	if mode == "" {
		mode = models.RegistrationModeClosed
	}
	slug := req.Slug
	if slug == "" {
		slug = GenerateTenantSlug(req.Name)
	}
	status := req.Status
	if status == "" {
		status = models.TenantStatusActive
	}
	tenant := &models.Tenant{
		Name:             req.Name,
		Slug:             slug,
		Domain:           normalizeTenantDomain(req.Domain),
		Status:           status,
		IsDefault:        req.Default,
		Settings:         req.Settings,
		RegistrationMode: mode,
	}

	// Validate tenant
	if err := tenant.ValidateWithContext(ctx); err != nil {
		return nil, fmt.Errorf("tenant validation failed: %w", err)
	}
	if err := s.checkTenantDomainFree(ctx, "", tenant.Domain); err != nil {
		return nil, err
	}

	// Create tenant using factory set
	createdTenant, err := s.factorySet.TenantRegistry.Create(ctx, *tenant)
	if err != nil {
		return nil, fmt.Errorf("failed to create tenant: %w", tenantDomainTaken(err, tenant.Domain))
	}

	return createdTenant, nil
}

// GetTenant retrieves a tenant by ID or slug. Only a tenant that matches
// neither is registry.ErrNotFound; a failed lookup is returned as it is.
func (s *Service) GetTenant(ctx context.Context, idOrSlug string) (*models.Tenant, error) {
	tenant, err := s.factorySet.TenantRegistry.Get(ctx, idOrSlug)
	if err == nil {
		return tenant, nil
	}
	if !errors.Is(err, registry.ErrNotFound) {
		return nil, errxtrace.Wrap("failed to look the tenant up by ID", err)
	}
	tenant, err = s.factorySet.TenantRegistry.GetBySlug(ctx, idOrSlug)
	if err == nil {
		return tenant, nil
	}
	if !errors.Is(err, registry.ErrNotFound) {
		return nil, errxtrace.Wrap("failed to look the tenant up by slug", err)
	}
	return nil, errxtrace.ClassifyNew(fmt.Sprintf("tenant '%s' not found (tried both ID and slug)", idOrSlug), registry.ErrNotFound)
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

	// Apply pagination. A non-positive Limit means "no limit", not "an empty
	// page": callers that only want the count would otherwise get a
	// TotalCount alongside zero rows (#2131).
	totalCount := len(filteredTenants)
	start := min(req.Offset, len(filteredTenants))
	end := len(filteredTenants)
	if req.Limit > 0 {
		end = min(start+req.Limit, len(filteredTenants))
	}

	paginatedTenants := filteredTenants[start:end]

	return &TenantListResponse{
		Tenants:    paginatedTenants,
		TotalCount: totalCount,
	}, nil
}

// UpdateTenant updates a tenant. Only the fields req sets change; an empty
// Domain clears the domain. A slug another tenant already has is refused
// with registry.ErrSlugAlreadyExists, and a new domain another tenant
// already has with ErrTenantDomainTaken.
//
// A change of status away from active ends the sessions of every user in the
// tenant, as SuspendTenant does. When the error wraps
// ErrTenantSessionsNotEnded the update was saved and the returned tenant is
// the saved row.
func (s *Service) UpdateTenant(ctx context.Context, idOrSlug string, req TenantUpdateRequest) (*models.Tenant, error) {
	// Get existing tenant
	tenant, err := s.GetTenant(ctx, idOrSlug)
	if err != nil {
		return nil, err
	}
	previousStatus := tenant.Status
	previousDomain := tenant.Domain

	// Apply updates
	if req.Name != nil {
		tenant.Name = *req.Name
	}
	if req.Slug != nil {
		tenant.Slug = *req.Slug
	}
	if req.Domain != nil {
		tenant.Domain = normalizeTenantDomain(req.Domain)
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
	// Only a domain the update changes is checked. The unique index on
	// tenants.domain keeps the stored one unique.
	if !sameTenantDomain(previousDomain, tenant.Domain) {
		if err := s.checkTenantDomainFree(ctx, tenant.ID, tenant.Domain); err != nil {
			return nil, err
		}
	}

	// Update tenant
	updatedTenant, err := s.factorySet.TenantRegistry.Update(ctx, *tenant)
	if err != nil {
		return nil, fmt.Errorf("failed to update tenant: %w", tenantDomainTaken(err, tenant.Domain))
	}

	if previousStatus == models.TenantStatusActive && updatedTenant.Status != models.TenantStatusActive {
		return updatedTenant, s.EndTenantSessions(ctx, updatedTenant.ID)
	}
	return updatedTenant, nil
}

// DeleteTenant hard-deletes a tenant and every row that depends on it,
// mirroring the GroupPurgeService orchestration order (#2115).
//
// Order (each step aborts the whole operation on error):
//
//	a) check that the row purge and the tenants DELETE can succeed
//	   (TenantPurger.CheckTenantPurge, a rolled-back dry run), so a foreign
//	   key the purge misses stops the deletion before any blob is removed.
//	b) delete the tenant's physical blobs. They go before the rows because the
//	   shared-key guard of the blob delete reads the tenant's file rows. A dead
//	   object store stops the deletion with every row in place. If no
//	   fileService was wired, blob cleanup is skipped with a loud WARNING and
//	   the blobs are orphaned — the DB purge still proceeds.
//	c) purge every tenant-scoped dependent row (TenantPurger), in FK-safe order.
//	d) drop the now-childless tenants row itself.
//
// Every step is idempotent, so calling DeleteTenant again after a failure, or
// after the process stopped half-way, continues the deletion: blobs already
// removed are skipped and the rows go as before.
func (s *Service) DeleteTenant(ctx context.Context, idOrSlug string) error {
	// Validate the tenant exists (and resolve slug -> id).
	tenant, err := s.GetTenant(ctx, idOrSlug)
	if err != nil {
		return err
	}

	// (a) Dry run of the row purge, before anything is removed.
	if err := s.factorySet.TenantPurger.CheckTenantPurge(ctx, tenant.ID); err != nil {
		return errxtrace.Wrap("the tenant's rows cannot be purged, so no blob was deleted", err)
	}

	// (b) Physical blobs, while the file rows the shared-key guard reads are
	// still there.
	if s.fileService != nil {
		if err := s.fileService.DeletePhysicalFilesForTenant(ctx, tenant.ID); err != nil {
			return errxtrace.Wrap("failed to delete tenant physical blobs", err)
		}
	} else {
		slog.WarnContext(ctx,
			"DeleteTenant: no file service wired — skipping physical blob cleanup; object-storage blobs for this tenant will be orphaned",
			"tenant_id", tenant.ID,
			"tenant_slug", tenant.Slug,
		)
	}

	// (c) Purge every tenant-scoped dependent row in FK-safe order.
	if err := s.factorySet.TenantPurger.PurgeTenantDependents(ctx, tenant.ID); err != nil {
		return errxtrace.Wrap("failed to purge tenant dependents", err)
	}

	// (d) Drop the now-childless tenants row.
	if err := s.factorySet.TenantRegistry.Delete(ctx, tenant.ID); err != nil {
		return errxtrace.Wrap("failed to delete tenant row", err)
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

	// Apply pagination. Non-positive Limit means unlimited — see ListTenants.
	totalCount := len(filteredUsers)
	start := min(req.Offset, len(filteredUsers))
	end := len(filteredUsers)
	if req.Limit > 0 {
		end = min(start+req.Limit, len(filteredUsers))
	}

	paginatedUsers := filteredUsers[start:end]

	return &UserListResponse{
		Users:      paginatedUsers,
		TotalCount: totalCount,
	}, nil
}

// UpdateUser updates a user. Only the fields set in req change.
//
// A TenantID that differs from the user's tenant moves the user (#2179). The
// move is allowed only for a user who has no group memberships and owns or
// authored nothing in any tenant-scoped table; otherwise it returns a
// *registry.UserTenantMoveBlockedError listing what blocks it, and nothing
// changes, the other fields of req included. A move ends every session and
// deletes the user's API tokens, keeps the password, MFA and settings, and
// copies the profile photo into the new tenant's storage.
func (s *Service) UpdateUser(ctx context.Context, idOrEmail string, req UserUpdateRequest) (*models.User, error) {
	// Get existing user
	user, err := s.GetUser(ctx, idOrEmail)
	if err != nil {
		return nil, err
	}

	// Apply and validate the field changes first, so an invalid request is
	// refused before a move changes anything.
	updated := *user
	if err := applyUserFields(ctx, &updated, req); err != nil {
		return nil, err
	}

	if req.TenantID != nil && *req.TenantID != user.TenantID {
		moved, err := s.moveUserToTenant(ctx, user, *req.TenantID)
		if err != nil {
			return nil, err
		}
		// The move rewrote tenant_id, default_group_id, avatar_path and
		// updated_at; the field changes go on top of that row.
		updated.TenantID = moved.TenantID
		updated.DefaultGroupID = moved.DefaultGroupID
		updated.AvatarPath = moved.AvatarPath
		updated.UpdatedAt = moved.UpdatedAt
		if !hasUserFieldChanges(req) {
			return moved, nil
		}
	}

	// Update user
	updatedUser, err := s.factorySet.UserRegistry.Update(ctx, updated)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Deactivating the account or setting its password ends its personal API
	// tokens (#2808), the same as the matching self-service and back-office
	// actions do. Token authentication would refuse an inactive owner anyway;
	// revoking makes it permanent, so reactivating the account later does not
	// bring the tokens back. A tenant move deletes the tokens itself.
	deactivated := req.IsActive != nil && !*req.IsActive
	if (deactivated || req.Password != nil) && s.factorySet.APITokenRegistry != nil {
		if _, err := s.factorySet.APITokenRegistry.RevokeAllByUserID(ctx, updatedUser.ID); err != nil {
			return nil, fmt.Errorf("user updated, but revoking their API tokens failed: %w", err)
		}
	}

	return updatedUser, nil
}

// applyUserFields applies the non-tenant fields of req to user and validates
// the result.
func applyUserFields(ctx context.Context, user *models.User, req UserUpdateRequest) error {
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}
	if req.Password != nil {
		if err := user.SetPassword(*req.Password); err != nil {
			return fmt.Errorf("failed to set password: %w", err)
		}
	}
	if err := user.ValidateWithContext(ctx); err != nil {
		return fmt.Errorf("user validation failed: %w", err)
	}
	return nil
}

func hasUserFieldChanges(req UserUpdateRequest) bool {
	return req.Email != nil || req.Name != nil || req.IsActive != nil || req.Password != nil
}

// CheckUserTenantMove runs the checks a move of the user to tenantID runs
// before it writes anything, and changes nothing. It returns nil when the move
// would go through, a *registry.UserTenantMoveBlockedError when the user still
// belongs to a group or owns content, and otherwise the error the move would
// stop with: the target tenant is missing or not active, its users already
// have the email, or the profile photo is not in the avatar storage.
func (s *Service) CheckUserTenantMove(ctx context.Context, idOrEmail, tenantID string) error {
	user, err := s.GetUser(ctx, idOrEmail)
	if err != nil {
		return err
	}
	return s.checkTenantMove(ctx, user, tenantID)
}

func (s *Service) checkTenantMove(ctx context.Context, user *models.User, tenantID string) error {
	mover := s.factorySet.UserTenantMover
	if mover == nil {
		return errxtrace.Wrap("user tenant mover is not configured", registry.ErrInvalidConfig)
	}
	target, err := s.factorySet.TenantRegistry.Get(ctx, tenantID)
	if err != nil {
		return errxtrace.Wrap("failed to find the target tenant", err)
	}
	if target.Status != models.TenantStatusActive {
		return errxtrace.Classify(registry.ErrTenantNotActive, errx.Attrs("tenant_id", tenantID, "status", target.Status))
	}
	switch other, err := s.factorySet.UserRegistry.GetByEmail(ctx, tenantID, user.Email); {
	case err == nil && other.ID != user.ID:
		return errxtrace.Classify(registry.ErrEmailAlreadyExists, errx.Attrs("email", user.Email, "tenant_id", tenantID))
	case err != nil && !errors.Is(err, registry.ErrNotFound):
		return errxtrace.Wrap("failed to check the email in the target tenant", err)
	}

	owned, err := mover.CountOwnedContent(ctx, user.ID)
	if err != nil {
		return err
	}
	if !owned.IsEmpty() {
		return &registry.UserTenantMoveBlockedError{Owned: owned}
	}

	if user.AvatarPath == nil || *user.AvatarPath == "" {
		return nil
	}
	if s.avatars == nil {
		return errxtrace.Wrap("the user has a profile photo, and no avatar storage is configured to copy it to the new tenant", registry.ErrInvalidConfig)
	}
	if err := s.avatars.CheckStored(ctx, *user.AvatarPath); err != nil {
		return errxtrace.Wrap("the profile photo cannot be copied to the new tenant", err)
	}
	return nil
}

// moveUserToTenant moves user to tenantID through the registry's
// UserTenantMover. It runs checkTenantMove first, so a move that would be
// refused stops before the photo is copied; the mover checks again under its
// lock. The photo copy is removed when the move does not happen and the
// original after it does.
func (s *Service) moveUserToTenant(ctx context.Context, user *models.User, tenantID string) (moved *models.User, err error) {
	defer func() {
		s.logAdminAction(ctx, "admin.move_user_tenant", &user.TenantID, user.ID, err)
	}()

	if err := s.checkTenantMove(ctx, user, tenantID); err != nil {
		return nil, err
	}

	hasAvatar := user.AvatarPath != nil && *user.AvatarPath != ""
	var avatarCopy *string
	if hasAvatar {
		if avatarCopy, err = s.avatars.CopyToTenant(ctx, user, tenantID); err != nil {
			return nil, errxtrace.Wrap("failed to copy the profile photo to the new tenant", err)
		}
	}

	moved, err = s.factorySet.UserTenantMover.MoveUserToTenant(ctx, registry.UserTenantMove{
		UserID:         user.ID,
		FromTenantID:   user.TenantID,
		ToTenantID:     tenantID,
		FromAvatarPath: user.AvatarPath,
		AvatarPath:     avatarCopy,
	})
	if err != nil {
		if avatarCopy != nil {
			s.avatars.DeleteStored(ctx, *avatarCopy)
		}
		if _, blocked := errors.AsType[*registry.UserTenantMoveBlockedError](err); blocked {
			return nil, err
		}
		return nil, errxtrace.Wrap("failed to move the user to another tenant", err)
	}
	if hasAvatar {
		s.avatars.DeleteStored(ctx, *user.AvatarPath)
	}
	return moved, nil
}

// ErrUserOwnsContent is returned by DeleteUser for a user who still authored
// content deleting the account cannot remove or hand over: items, files,
// places, exports, or a group they created. The delete changes nothing.
var ErrUserOwnsContent = errx.NewSentinel("the user still owns content")

// ErrUserSoleOwnerOfSharedGroup is returned by DeleteUser for a user who is
// the only owner of a group that has other members: the delete would leave
// that group with members and no owner. It is the rule of self-service
// account deletion, so it matches services.ErrAccountSoleOwnerOfSharedGroup
// too. The delete changes nothing.
var ErrUserSoleOwnerOfSharedGroup = errx.NewSentinel("the user is the only owner of a shared group", services.ErrAccountSoleOwnerOfSharedGroup)

// DeleteUser hard-deletes a user's account along with its auth / identity rows.
//
// Order:
//
//	a) refuse, before anything is changed, a user who is the only owner of a
//	   group that has other members (as self-service account deletion
//	   does), and a user who still authored content (items, files, places,
//	   exports, groups they created) in any group: those rows carry a NOT
//	   NULL created_by_user_id (or created_by) and cannot be orphaned. Unlike
//	   self-service account deletion, an operator's delete purges no group,
//	   so content in the user's private groups counts too;
//	b) purge the user's auth/identity dependent rows (UserPurger): refresh
//	   tokens, login events, MFA secret, OAuth identities, memberships, …,
//	   and hand the group rows they created to another member;
//	c) drop the users row itself.
func (s *Service) DeleteUser(ctx context.Context, idOrEmail string) error {
	// Validate the user exists (and resolve email -> id + tenant).
	user, err := s.GetUser(ctx, idOrEmail)
	if err != nil {
		return err
	}

	// (a) Removing the memberships in (b) would leave a group the user
	// alone owns with members and no owner, and the final delete would fail
	// on owned content after (b) had removed the user from every group. Ask
	// first, so a refused delete leaves the user as they were.
	soleOwnedGroupID, err := services.SoleOwnedSharedGroup(ctx, s.factorySet.GroupMembershipRegistry, user.TenantID, user.ID)
	if err != nil {
		return errxtrace.Wrap("failed to check the user's groups", err)
	}
	if soleOwnedGroupID != "" {
		return s.soleOwnerError(ctx, user, soleOwnedGroupID)
	}
	if s.factorySet.UserContentOwnershipChecker == nil {
		return errxtrace.Wrap("the content ownership check is not configured", registry.ErrFieldRequired)
	}
	owns, err := s.factorySet.UserContentOwnershipChecker.HasRetainedOwnedContent(ctx, user.TenantID, user.ID, nil)
	if err != nil {
		return errxtrace.Wrap("failed to check the user's content", err)
	}
	if owns {
		return userOwnsContentError(user)
	}

	// (b) Purge the user's auth / identity dependent rows so the parent
	// users row has no remaining NO ACTION children from those tables.
	if err := s.factorySet.UserPurger.PurgeUserDependents(ctx, user.TenantID, user.ID); err != nil {
		return errxtrace.Wrap("failed to purge user dependents", err)
	}

	// (c) Drop the users row. Content created between (a) and here is the
	// one way the foreign keys can still reject it.
	if err := s.factorySet.UserRegistry.Delete(ctx, user.ID); err != nil {
		return errxtrace.Wrap("failed to delete the user row", err)
	}

	// (d) The account is gone, so a photo left behind would be a personal
	// image nobody can see or remove. Best effort, as for self-service
	// account deletion: the user is already deleted.
	if s.avatars != nil && user.AvatarPath != nil {
		s.avatars.DeleteStored(ctx, *user.AvatarPath)
	}

	return nil
}

// userOwnsContentError tells the operator what keeps the user and what to do.
func userOwnsContentError(user *models.User) error {
	return errxtrace.Wrap(
		fmt.Sprintf(
			"cannot delete user %s (%s): they still own content (items, files, places, exports or groups they created) — reassign or delete that content first",
			user.ID, user.Email,
		),
		ErrUserOwnsContent,
	)
}

// soleOwnerError names the group the user alone owns, so the operator knows
// whom to make an owner. The name is best effort: a group that cannot be read
// is named by its ID only.
func (s *Service) soleOwnerError(ctx context.Context, user *models.User, groupID string) error {
	group := groupID
	if g, err := s.factorySet.LocationGroupRegistry.Get(ctx, groupID); err == nil {
		group = fmt.Sprintf("%q (%s)", g.Name, g.ID)
	}
	return errxtrace.Wrap(
		fmt.Sprintf(
			"cannot delete user %s (%s): they are the only owner of the group %s, which has other members — make another member an owner first",
			user.ID, user.Email, group,
		),
		ErrUserSoleOwnerOfSharedGroup,
	)
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
		// returning so the trail records the attempt regardless. The
		// subject user is already resolved, so include it for forensics.
		s.logAdminAction(ctx, "admin.revoke_system_admin", &user.TenantID, user.ID, configErr)
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

// Worker soft-pause action names (#1308). Kept as constants so the CLI
// audit rows use the same literals as the admin REST surface
// (admin.worker_pause / admin.worker_resume), mirroring the
// "admin.<noun>_<verb>" pattern set by #1745.
const (
	// auditActionWorkerPause is the audit-row Action emitted when an
	// operator soft-pauses a background worker via the CLI.
	auditActionWorkerPause = "admin.worker_pause"
	// auditActionWorkerResume is the audit-row Action emitted when an
	// operator resumes a soft-paused background worker via the CLI.
	auditActionWorkerResume = "admin.worker_resume"
	// auditActionListWorkerControls is the audit-row Action emitted on a
	// read of the worker-control listing, so operator-side reads land in
	// the trail alongside the writes (mirrors admin.list_system_admins).
	auditActionListWorkerControls = "admin.list_worker_controls"
)

// ErrUnknownWorkerType is returned by PauseWorker/ResumeWorker when the
// supplied worker type is not one of models.AllWorkerTypes(). Surfaced
// to the CLI so a typo (`--type expot`) fails with a clear message
// rather than silently creating a control row for a non-existent worker.
var ErrUnknownWorkerType = errx.NewSentinel("unknown worker type")

// PauseWorker soft-pauses the named background worker (#1308). The
// workerType is validated against models.AllWorkerTypes() up front so a
// typo fails clearly rather than writing a control row for a worker that
// doesn't exist. Idempotent at the registry layer: re-pausing an
// already-paused worker updates by/reason but preserves the original
// paused_at. Writes an `admin.worker_pause` audit row regardless of
// outcome so the trail records the attempt.
//
// The CLI runs out-of-band with no authenticated operator session, so
// the control row's paused_by is recorded as "cli" — a coarse marker
// distinguishing CLI pauses from the admin-REST pauses (which record the
// back-office operator's id/email). The audit row's actor (UserID) stays
// nil for the same reason GrantSystemAdmin's does: CLI "operator"
// identity is the OS/host boundary, not a row in this database.
func (s *Service) PauseWorker(ctx context.Context, workerType, reason string) (*models.WorkerControl, error) {
	wt, ok := models.ParseWorkerType(workerType)
	if !ok {
		err := errxtrace.Classify(ErrUnknownWorkerType, errx.Attrs("worker_type", workerType))
		s.logWorkerAction(ctx, auditActionWorkerPause, workerType, err)
		return nil, err
	}

	if s.factorySet.WorkerControlRegistry == nil {
		configErr := errxtrace.Classify(registry.ErrInvalidConfig, errx.Attrs("missing", "WorkerControlRegistry"))
		s.logWorkerAction(ctx, auditActionWorkerPause, string(wt), configErr)
		return nil, configErr
	}

	control, err := s.factorySet.WorkerControlRegistry.Pause(ctx, string(wt), workerPausedByCLI, reason)
	if err != nil {
		s.logWorkerAction(ctx, auditActionWorkerPause, string(wt), err)
		return nil, errxtrace.Wrap("failed to pause worker", err)
	}

	s.logWorkerAction(ctx, auditActionWorkerPause, string(wt), nil)
	return control, nil
}

// ResumeWorker clears the soft-pause on the named worker (#1308).
// Idempotent: resuming a worker that is not paused (no control row) is a
// no-op that returns a synthetic not-paused row. Validates workerType up
// front and writes an `admin.worker_resume` audit row.
func (s *Service) ResumeWorker(ctx context.Context, workerType string) (*models.WorkerControl, error) {
	wt, ok := models.ParseWorkerType(workerType)
	if !ok {
		err := errxtrace.Classify(ErrUnknownWorkerType, errx.Attrs("worker_type", workerType))
		s.logWorkerAction(ctx, auditActionWorkerResume, workerType, err)
		return nil, err
	}

	if s.factorySet.WorkerControlRegistry == nil {
		configErr := errxtrace.Classify(registry.ErrInvalidConfig, errx.Attrs("missing", "WorkerControlRegistry"))
		s.logWorkerAction(ctx, auditActionWorkerResume, string(wt), configErr)
		return nil, configErr
	}

	control, err := s.factorySet.WorkerControlRegistry.Resume(ctx, string(wt))
	if err != nil {
		s.logWorkerAction(ctx, auditActionWorkerResume, string(wt), err)
		return nil, errxtrace.Wrap("failed to resume worker", err)
	}

	s.logWorkerAction(ctx, auditActionWorkerResume, string(wt), nil)
	return control, nil
}

// ListWorkerControls returns every worker_control row (#1308). The CLI
// `workers status` command merges these against models.AllWorkerTypes()
// so a worker with no row renders as "running". Logs an
// `admin.list_worker_controls` audit row regardless of result count so
// operator-side reads are visible in the trail too.
func (s *Service) ListWorkerControls(ctx context.Context) ([]*models.WorkerControl, error) {
	if s.factorySet.WorkerControlRegistry == nil {
		err := errxtrace.Classify(registry.ErrInvalidConfig, errx.Attrs("missing", "WorkerControlRegistry"))
		s.logWorkerAction(ctx, auditActionListWorkerControls, "", err)
		return nil, err
	}

	controls, err := s.factorySet.WorkerControlRegistry.List(ctx)
	if err != nil {
		s.logWorkerAction(ctx, auditActionListWorkerControls, "", err)
		return nil, errxtrace.Wrap("failed to list worker controls", err)
	}

	s.logWorkerAction(ctx, auditActionListWorkerControls, "", nil)
	return controls, nil
}

// workerPausedByCLI is the paused_by marker recorded for CLI-driven
// pauses (#1308). The CLI has no authenticated operator session, so a
// coarse "cli" marker distinguishes a CLI pause from an admin-REST pause
// (which records the back-office operator's id/email) without pretending
// a specific person did it.
const workerPausedByCLI = "cli"

// logWorkerAction writes a worker-control admin audit row. Mirrors
// logAdminAction but stores the worker type as the subject (EntityType
// "worker" / EntityID the worker-type string) rather than a user id.
// Best-effort: a missing audit row is recoverable and surfaced via slog.
// Actor (UserID) is nil for the same reason logAdminAction's is — CLI
// invocations have no authenticated operator row in this database.
func (s *Service) logWorkerAction(ctx context.Context, action, workerType string, opErr error) {
	if s.factorySet == nil || s.factorySet.AuditLogRegistry == nil {
		return
	}

	entry := models.AuditLog{
		Action:  action,
		UserID:  nil, // CLI invocations have no authenticated actor — see logAdminAction doc.
		Success: opErr == nil,
	}
	if workerType != "" {
		subjectType := "worker"
		entry.EntityType = &subjectType
		entry.EntityID = &workerType
	}
	if opErr != nil {
		msg := opErr.Error()
		entry.ErrorMessage = &msg
	}

	if _, createErr := s.factorySet.AuditLogRegistry.Create(ctx, entry); createErr != nil {
		slog.ErrorContext(ctx, "Failed to write worker-control audit log entry",
			"action", action, "worker_type", workerType, "error", createErr)
	}
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
		slog.ErrorContext(ctx, "Failed to write admin audit log entry",
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
