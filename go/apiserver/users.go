package apiserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// UsersAPI handles admin user management endpoints.
type UsersAPI struct {
	userRegistry registry.UserRegistry
	auditService services.AuditLogger
}

// AdminUserCreateRequest is the body for POST /users.
type AdminUserCreateRequest struct {
	Email    string          `json:"email"`
	Password string          `json:"password"`
	Name     string          `json:"name"`
	Role     models.UserRole `json:"role"`
	IsActive *bool           `json:"is_active,omitempty"`
}

// AdminUserUpdateRequest is the body for PUT /users/:id.
type AdminUserUpdateRequest struct {
	Email    *string          `json:"email,omitempty"`
	Name     *string          `json:"name,omitempty"`
	Role     *models.UserRole `json:"role,omitempty"`
	IsActive *bool            `json:"is_active,omitempty"`
	Password *string          `json:"password,omitempty"`
}

// AdminUserListResponse is the response for GET /users.
type AdminUserListResponse struct {
	Users      []*models.User `json:"users"`
	Total      int            `json:"total"`
	Page       int            `json:"page"`
	PerPage    int            `json:"per_page"`
	TotalPages int            `json:"total_pages"`
}

// RequireAdmin is middleware that ensures the authenticated user has the admin role.
func RequireAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := appctx.UserFromContext(r.Context())
			if user == nil {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}
			if user.Role != models.UserRoleAdmin {
				http.Error(w, "Admin access required", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// listUsers lists all users in the admin's tenant with optional filtering and pagination.
//
// @Summary List users (admin only)
// @Description List all users within the admin's tenant. Supports filtering by role, active status, and search.
// @Tags admin,users
// @Produce json
// @Param role query string false "Filter by role (admin|user)"
// @Param active query string false "Filter by active status (true|false)"
// @Param search query string false "Search by email or name"
// @Param page query int false "Page number (default 1)"
// @Param per_page query int false "Items per page (default 20, max 100)"
// @Success 200 {object} AdminUserListResponse "OK"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Router /users [get]
func (api *UsersAPI) listUsers(w http.ResponseWriter, r *http.Request) {
	currentUser := appctx.UserFromContext(r.Context())
	if currentUser == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	q := r.URL.Query()
	roleFilter := q.Get("role")
	activeFilter := strings.ToLower(q.Get("active"))
	searchFilter := strings.ToLower(q.Get("search"))

	if roleFilter != "" && roleFilter != string(models.UserRoleAdmin) && roleFilter != string(models.UserRoleUser) {
		http.Error(w, "Invalid role filter", http.StatusBadRequest)
		return
	}
	if activeFilter != "" && activeFilter != "true" && activeFilter != "false" {
		http.Error(w, "Invalid active filter", http.StatusBadRequest)
		return
	}

	page, perPage := parsePagination(q.Get("page"), q.Get("per_page"))

	users, err := api.userRegistry.ListByTenant(r.Context(), currentUser.TenantID)
	if err != nil {
		slog.Error("Failed to list users", "error", err, "tenant_id", currentUser.TenantID)
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}

	filtered := filterUsers(users, roleFilter, activeFilter, searchFilter)

	total := len(filtered)
	totalPages := (total + perPage - 1) / perPage
	if totalPages == 0 {
		totalPages = 1
	}
	start := (page - 1) * perPage
	if start > total {
		start = total
	}
	end := start + perPage
	if end > total {
		end = total
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(AdminUserListResponse{
		Users:      filtered[start:end],
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}

// getUser returns a single user by ID, enforcing tenant isolation.
//
// @Summary Get a user (admin only)
// @Description Retrieve a user by ID within the admin's tenant.
// @Tags admin,users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} models.User "OK"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Not Found"
// @Router /users/{id} [get]
func (api *UsersAPI) getUser(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	currentUser := appctx.UserFromContext(r.Context())
	if currentUser == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	user, ok := api.fetchUserInTenant(w, r, chi.URLParam(r, "id"), currentUser.TenantID)
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(user); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}

// createUser creates a new user within the admin's tenant.
//
// @Summary Create a user (admin only)
// @Tags admin,users
// @Accept json
// @Produce json
// @Param data body AdminUserCreateRequest true "User data"
// @Success 201 {object} models.User "Created"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 409 {string} string "Conflict - email already exists"
// @Router /users [post]
func (api *UsersAPI) createUser(w http.ResponseWriter, r *http.Request) {
	currentUser := appctx.UserFromContext(r.Context())
	if currentUser == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var req AdminUserCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errMsg := "invalid request body"
		api.logAdminAction(r, "admin_create_user", &currentUser.ID, &currentUser.TenantID, false, &errMsg)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.TrimSpace(req.Name)

	if req.Email == "" || req.Password == "" || req.Name == "" {
		errMsg := "missing required fields"
		api.logAdminAction(r, "admin_create_user", &currentUser.ID, &currentUser.TenantID, false, &errMsg)
		http.Error(w, "Email, password, and name are required", http.StatusBadRequest)
		return
	}
	if req.Role == "" {
		req.Role = models.UserRoleUser
	}
	if err := req.Role.Validate(); err != nil {
		errMsg := "invalid role"
		api.logAdminAction(r, "admin_create_user", &currentUser.ID, &currentUser.TenantID, false, &errMsg)
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}
	if err := models.ValidatePassword(req.Password); err != nil {
		errMsg := "invalid password"
		api.logAdminAction(r, "admin_create_user", &currentUser.ID, &currentUser.TenantID, false, &errMsg)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: currentUser.TenantID},
		Email:               req.Email,
		Name:                req.Name,
		Role:                req.Role,
		IsActive:            isActive,
	}
	if err := user.SetPassword(req.Password); err != nil {
		slog.Error("Failed to hash password", "error", err)
		errMsg := "failed to process password"
		api.logAdminAction(r, "admin_create_user", &currentUser.ID, &currentUser.TenantID, false, &errMsg)
		http.Error(w, "Failed to process password", http.StatusInternalServerError)
		return
	}
	if err := user.ValidateWithContext(r.Context()); err != nil {
		errMsg := "validation failed"
		api.logAdminAction(r, "admin_create_user", &currentUser.ID, &currentUser.TenantID, false, &errMsg)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	created, err := api.userRegistry.Create(r.Context(), *user)
	if err != nil {
		if errors.Is(err, registry.ErrEmailAlreadyExists) {
			errMsg := "email already exists"
			api.logAdminAction(r, "admin_create_user", &currentUser.ID, &currentUser.TenantID, false, &errMsg)
			http.Error(w, "User with this email already exists", http.StatusConflict)
			return
		}
		slog.Error("Failed to create user", "error", err)
		errMsg := "failed to create user"
		api.logAdminAction(r, "admin_create_user", &currentUser.ID, &currentUser.TenantID, false, &errMsg)
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	api.logAdminAction(r, "admin_create_user", &currentUser.ID, &currentUser.TenantID, true, nil)
	slog.Info("Admin created user", "admin_id", currentUser.ID, "new_user_id", created.ID, "email", created.Email)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(created); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}

// updateUser updates fields of a user within the admin's tenant.
//
// @Summary Update a user (admin only)
// @Tags admin,users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param data body AdminUserUpdateRequest true "Fields to update"
// @Success 200 {object} models.User "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Not Found"
// @Router /users/{id} [put]
func (api *UsersAPI) updateUser(w http.ResponseWriter, r *http.Request) {
	currentUser := appctx.UserFromContext(r.Context())
	if currentUser == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	userID := chi.URLParam(r, "id")
	user, ok := api.fetchUserInTenant(w, r, userID, currentUser.TenantID)
	if !ok {
		errMsg := "user not found"
		api.logAdminAction(r, "admin_update_user", &currentUser.ID, &currentUser.TenantID, false, &errMsg)
		return
	}

	req, decodeErr := decodeAdminUserUpdateRequest(r)
	if decodeErr != nil {
		errMsg := "invalid request body"
		api.logAdminAction(r, "admin_update_user", &currentUser.ID, &currentUser.TenantID, false, &errMsg)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if selfErr := validateAdminSelfUpdate(userID, currentUser.ID, req); selfErr != nil {
		api.logAdminAction(r, "admin_update_user", &currentUser.ID, &currentUser.TenantID, false, &selfErr.auditMsg)
		http.Error(w, selfErr.clientMsg, selfErr.status)
		return
	}

	updated, updErr := api.applyAdminUserUpdate(r, currentUser, user, req)
	if updErr != nil {
		api.logAdminAction(r, "admin_update_user", &currentUser.ID, &currentUser.TenantID, false, &updErr.auditMsg)
		http.Error(w, updErr.clientMsg, updErr.status)
		return
	}

	api.logAdminAction(r, "admin_update_user", &currentUser.ID, &currentUser.TenantID, true, nil)
	slog.Info("Admin updated user", "admin_id", currentUser.ID, "user_id", updated.ID)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(updated); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}

// adminUpdateError is a small transport for user-friendly HTTP errors.
type adminUpdateError struct {
	status    int
	clientMsg string
	auditMsg  string
}

func decodeAdminUserUpdateRequest(r *http.Request) (AdminUserUpdateRequest, error) {
	var req AdminUserUpdateRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	return req, err
}

func validateAdminSelfUpdate(targetUserID, currentUserID string, req AdminUserUpdateRequest) *adminUpdateError {
	if targetUserID != currentUserID {
		return nil
	}
	if req.IsActive != nil && !*req.IsActive {
		return &adminUpdateError{
			status:    http.StatusBadRequest,
			clientMsg: "Cannot deactivate your own account",
			auditMsg:  "cannot deactivate own account",
		}
	}
	if req.Role != nil && *req.Role != models.UserRoleAdmin {
		return &adminUpdateError{
			status:    http.StatusBadRequest,
			clientMsg: "Cannot change your own role",
			auditMsg:  "cannot change own role",
		}
	}
	return nil
}

func (api *UsersAPI) applyAdminUserUpdate(r *http.Request, currentUser, user *models.User, req AdminUserUpdateRequest) (*models.User, *adminUpdateError) {
	ctx := r.Context()

	if req.Email != nil {
		normalizedEmail := strings.ToLower(strings.TrimSpace(*req.Email))
		if normalizedEmail == "" {
			return nil, &adminUpdateError{
				status:    http.StatusBadRequest,
				clientMsg: "Email cannot be empty",
				auditMsg:  "email cannot be empty",
			}
		}

		// Pre-check to return 409 instead of a generic 500 on unique-index violations.
		// (Also ensures the email uniqueness rule remains tenant-scoped.)
		existing, err := api.userRegistry.GetByEmail(ctx, currentUser.TenantID, normalizedEmail)
		if err == nil && existing != nil && existing.ID != user.ID {
			return nil, &adminUpdateError{
				status:    http.StatusConflict,
				clientMsg: "User with this email already exists",
				auditMsg:  "email already exists",
			}
		}
		if err != nil && !errors.Is(err, registry.ErrNotFound) {
			slog.Error("Failed to check email uniqueness", "email", normalizedEmail, "error", err)
			return nil, &adminUpdateError{
				status:    http.StatusInternalServerError,
				clientMsg: "Failed to update user",
				auditMsg:  "failed to check email uniqueness",
			}
		}

		user.Email = normalizedEmail
	}

	if req.Name != nil {
		user.Name = strings.TrimSpace(*req.Name)
	}

	if req.Role != nil {
		if err := req.Role.Validate(); err != nil {
			return nil, &adminUpdateError{
				status:    http.StatusBadRequest,
				clientMsg: "Invalid role",
				auditMsg:  "invalid role",
			}
		}
		user.Role = *req.Role
	}

	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	if req.Password != nil {
		if err := models.ValidatePassword(*req.Password); err != nil {
			return nil, &adminUpdateError{
				status:    http.StatusBadRequest,
				clientMsg: err.Error(),
				auditMsg:  "invalid password",
			}
		}
		if err := user.SetPassword(*req.Password); err != nil {
			slog.Error("Failed to hash password", "error", err)
			return nil, &adminUpdateError{
				status:    http.StatusInternalServerError,
				clientMsg: "Failed to process password",
				auditMsg:  "failed to process password",
			}
		}
	}

	if err := user.ValidateWithContext(ctx); err != nil {
		return nil, &adminUpdateError{
			status:    http.StatusBadRequest,
			clientMsg: err.Error(),
			auditMsg:  "validation failed",
		}
	}

	updated, err := api.userRegistry.Update(ctx, *user)
	if err != nil {
		slog.Error("Failed to update user", "error", err)
		if errors.Is(err, registry.ErrEmailAlreadyExists) {
			return nil, &adminUpdateError{
				status:    http.StatusConflict,
				clientMsg: "User with this email already exists",
				auditMsg:  "email already exists",
			}
		}
		return nil, &adminUpdateError{
			status:    http.StatusInternalServerError,
			clientMsg: "Failed to update user",
			auditMsg:  "failed to update user",
		}
	}

	return updated, nil
}

// deactivateUser sets a user's is_active flag to false.
// It prevents an admin from deactivating their own account.
//
// @Summary Deactivate a user (admin only)
// @Tags admin,users
// @Param id path string true "User ID"
// @Success 200 {object} map[string]string "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Not Found"
// @Router /users/{id} [delete]
func (api *UsersAPI) deactivateUser(w http.ResponseWriter, r *http.Request) {
	currentUser := appctx.UserFromContext(r.Context())
	if currentUser == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	userID := chi.URLParam(r, "id")
	if userID == currentUser.ID {
		errMsg := "cannot deactivate own account"
		api.logAdminAction(r, "admin_deactivate_user", &currentUser.ID, &currentUser.TenantID, false, &errMsg)
		http.Error(w, "Cannot deactivate your own account", http.StatusBadRequest)
		return
	}

	user, ok := api.fetchUserInTenant(w, r, userID, currentUser.TenantID)
	if !ok {
		errMsg := "user not found"
		api.logAdminAction(r, "admin_deactivate_user", &currentUser.ID, &currentUser.TenantID, false, &errMsg)
		return
	}

	user.IsActive = false
	if _, err := api.userRegistry.Update(r.Context(), *user); err != nil {
		slog.Error("Failed to deactivate user", "error", err)
		errMsg := "failed to deactivate user"
		api.logAdminAction(r, "admin_deactivate_user", &currentUser.ID, &currentUser.TenantID, false, &errMsg)
		http.Error(w, "Failed to deactivate user", http.StatusInternalServerError)
		return
	}

	api.logAdminAction(r, "admin_deactivate_user", &currentUser.ID, &currentUser.TenantID, true, nil)
	slog.Info("Admin deactivated user", "admin_id", currentUser.ID, "user_id", userID)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "User deactivated successfully"}); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// fetchUserInTenant retrieves a user by ID and validates it belongs to the given tenant.
// It writes the appropriate HTTP error and returns false when the user cannot be fetched.
func (api *UsersAPI) fetchUserInTenant(w http.ResponseWriter, r *http.Request, userID, tenantID string) (*models.User, bool) {
	user, err := api.userRegistry.Get(r.Context(), userID)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			http.Error(w, "User not found", http.StatusNotFound)
			return nil, false
		}
		slog.Error("Failed to get user", "error", err)
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return nil, false
	}
	if user.TenantID != tenantID {
		// Return 404 instead of 403 to avoid leaking information about other tenants.
		http.Error(w, "User not found", http.StatusNotFound)
		return nil, false
	}
	return user, true
}

// parsePagination parses page and per_page query strings and returns safe defaults.
func parsePagination(pageStr, perPageStr string) (page, perPage int) {
	page = 1
	perPage = 20
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 && pp <= 100 {
		perPage = pp
	}
	return page, perPage
}

// filterUsers filters a slice of users by role, active status, and search string.
func filterUsers(users []*models.User, roleFilter, activeFilter, searchFilter string) []*models.User {
	result := make([]*models.User, 0, len(users))
	for _, u := range users {
		if roleFilter != "" && string(u.Role) != roleFilter {
			continue
		}
		if activeFilter != "" {
			if (activeFilter == "true") != u.IsActive {
				continue
			}
		}
		if searchFilter != "" {
			if !strings.Contains(strings.ToLower(u.Email), searchFilter) &&
				!strings.Contains(strings.ToLower(u.Name), searchFilter) {
				continue
			}
		}
		result = append(result, u)
	}
	return result
}

// logAdminAction writes an admin action to the audit log (best-effort, no-op when auditService is nil).
func (api *UsersAPI) logAdminAction(r *http.Request, action string, userID, tenantID *string, success bool, errMsg *string) {
	if api.auditService == nil {
		return
	}
	api.auditService.LogAuth(r.Context(), action, userID, tenantID, success, r, errMsg)
}

// -----------------------------------------------------------------------
// Route registration
// -----------------------------------------------------------------------

// UsersParams holds all dependencies needed by the admin users API.
type UsersParams struct {
	UserRegistry registry.UserRegistry
	AuditService services.AuditLogger
}

// Users returns a Chi router function that mounts admin user management routes.
// All routes require the requesting user to have the admin role.
func Users(params UsersParams) func(r chi.Router) {
	api := &UsersAPI{
		userRegistry: params.UserRegistry,
		auditService: params.AuditService,
	}

	return func(r chi.Router) {
		r.Use(RequireAdmin())
		r.Get("/", api.listUsers)
		r.Post("/", api.createUser)
		r.Get("/{id}", api.getUser)
		r.Put("/{id}", api.updateUser)
		r.Delete("/{id}", api.deactivateUser)
	}
}
