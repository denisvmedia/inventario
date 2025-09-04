package admin

import (
	"context"
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

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
	Name     string
	Slug     string
	Domain   *string
	Status   models.TenantStatus
	Settings map[string]any
	Default  bool
}

// TenantUpdateRequest represents a tenant update request
type TenantUpdateRequest struct {
	Name     *string
	Slug     *string
	Domain   *string
	Status   *models.TenantStatus
	Settings map[string]any
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
	tenant := &models.Tenant{
		Name:     req.Name,
		Slug:     req.Slug,
		Domain:   req.Domain,
		Status:   req.Status,
		Settings: req.Settings,
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

	// Handle default tenant setting
	if req.Default {
		// Implementation for setting default tenant would go here
		// This might involve updating system settings
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
	start := req.Offset
	if start > len(filteredTenants) {
		start = len(filteredTenants)
	}

	end := start + req.Limit
	if end > len(filteredTenants) {
		end = len(filteredTenants)
	}

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
	Role     models.UserRole
	IsActive bool
}

// UserUpdateRequest represents a user update request
type UserUpdateRequest struct {
	Email    *string
	Name     *string
	Role     *models.UserRole
	IsActive *bool
	TenantID *string
	Password *string
}

// UserListRequest represents a user list request
type UserListRequest struct {
	TenantID string
	Role     string
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
		Email:    req.Email,
		Name:     req.Name,
		TenantID: req.TenantID,
		Role:     req.Role,
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
	userRegistry := s.registrySet.UserRegistry()

	// Build filter criteria
	filter := make(map[string]any)
	if req.TenantID != "" {
		filter["tenant_id"] = req.TenantID
	}
	if req.Role != "" {
		filter["role"] = req.Role
	}
	if req.Active != nil {
		filter["is_active"] = *req.Active
	}
	if req.Search != "" {
		filter["search"] = req.Search
	}

	// Get users with pagination
	users, err := userRegistry.List(ctx, req.Limit, req.Offset, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	// Get total count
	totalCount, err := userRegistry.Count(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	return &UserListResponse{
		Users:      users,
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
	if req.Role != nil {
		user.Role = *req.Role
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}
	if req.TenantID != nil {
		user.TenantID = *req.TenantID
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
	userRegistry := s.registrySet.UserRegistry()
	updatedUser, err := userRegistry.Update(ctx, *user)
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
	userRegistry := s.registrySet.UserRegistry()
	if err := userRegistry.Delete(ctx, user.ID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
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
