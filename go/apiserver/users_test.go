package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// mockUserRegistryForUsersTests extends the security test mock with
// email-uniqueness enforcement needed by the admin create/update handlers.
type mockUserRegistryForUsersTests struct {
	mockUserRegistryForSecurityTests
}

func (m *mockUserRegistryForUsersTests) Create(ctx context.Context, user models.User) (*models.User, error) {
	for _, u := range m.users {
		if u.Email == user.Email && u.TenantID == user.TenantID {
			return nil, registry.ErrEmailAlreadyExists
		}
	}
	m.users[user.ID] = &user
	return &user, nil
}

// newUsersRouter builds a chi router wired with the Users routes.
// The provided currentUser is injected into every request context,
// simulating what JWTMiddleware does in production.
func newUsersRouter(currentUser *models.User, reg registry.UserRegistry) chi.Router {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := appctx.WithUser(r.Context(), currentUser)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Route("/users", apiserver.Users(apiserver.UsersParams{UserRegistry: reg}))
	return r
}

// -----------------------------------------------------------------------
// RequireAdmin middleware
// -----------------------------------------------------------------------

func TestUsersAPI_RequireAdmin_NonAdminGets403(t *testing.T) {
	tests := []struct {
		name   string
		role   models.UserRole
		method string
		path   string
	}{
		{"list as user", models.UserRoleUser, http.MethodGet, "/users"},
		{"get as user", models.UserRoleUser, http.MethodGet, "/users/some-id"},
		{"create as user", models.UserRoleUser, http.MethodPost, "/users"},
		{"update as user", models.UserRoleUser, http.MethodPut, "/users/some-id"},
		{"deactivate as user", models.UserRoleUser, http.MethodDelete, "/users/some-id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			caller := &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "caller-id"},
					TenantID: "tenant-a",
				},
				Role:     tt.role,
				IsActive: true,
			}
			reg := &mockUserRegistryForSecurityTests{users: map[string]*models.User{}}
			r := newUsersRouter(caller, reg)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			c.Assert(w.Code, qt.Equals, http.StatusForbidden)
		})
	}
}

// -----------------------------------------------------------------------
// Tenant isolation
// -----------------------------------------------------------------------

func TestUsersAPI_GetUser_CrossTenantReturns404(t *testing.T) {
	c := qt.New(t)

	otherTenantUser := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "other-user"},
			TenantID: "tenant-b",
		},
		Email:    "other@tenant-b.com",
		Name:     "Other User",
		Role:     models.UserRoleUser,
		IsActive: true,
	}
	admin := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "admin-id"},
			TenantID: "tenant-a",
		},
		Role:     models.UserRoleAdmin,
		IsActive: true,
	}
	reg := &mockUserRegistryForSecurityTests{
		users: map[string]*models.User{"other-user": otherTenantUser},
	}
	r := newUsersRouter(admin, reg)

	req := httptest.NewRequest(http.MethodGet, "/users/other-user", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusNotFound)
}



// -----------------------------------------------------------------------
// Create user – duplicate email
// -----------------------------------------------------------------------

func TestUsersAPI_CreateUser_DuplicateEmailReturns409(t *testing.T) {
	c := qt.New(t)

	existing := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "existing-id"},
			TenantID: "tenant-a",
		},
		Email:    "taken@example.com",
		Name:     "Existing User",
		Role:     models.UserRoleUser,
		IsActive: true,
	}
	admin := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "admin-id"},
			TenantID: "tenant-a",
		},
		Role:     models.UserRoleAdmin,
		IsActive: true,
	}
	reg := &mockUserRegistryForUsersTests{
		mockUserRegistryForSecurityTests: mockUserRegistryForSecurityTests{
			users: map[string]*models.User{"existing-id": existing},
		},
	}
	r := newUsersRouter(admin, reg)

	body, _ := json.Marshal(map[string]any{
		"email":    "taken@example.com",
		"password": "ValidPass123!",
		"name":     "New User",
		"role":     "user",
	})
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusConflict)
}

// -----------------------------------------------------------------------
// Deactivate user – self-deactivation
// -----------------------------------------------------------------------

func TestUsersAPI_DeactivateUser_SelfReturns400(t *testing.T) {
	c := qt.New(t)

	admin := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "admin-id"},
			TenantID: "tenant-a",
		},
		Role:     models.UserRoleAdmin,
		IsActive: true,
	}
	reg := &mockUserRegistryForSecurityTests{
		users: map[string]*models.User{"admin-id": admin},
	}
	r := newUsersRouter(admin, reg)

	req := httptest.NewRequest(http.MethodDelete, "/users/admin-id", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusBadRequest)
}

// -----------------------------------------------------------------------
// Update user – self-demotion / self-deactivation via PUT
// -----------------------------------------------------------------------

func TestUsersAPI_UpdateUser_SelfProtection(t *testing.T) {
	tests := []struct {
		name           string
		payload        map[string]any
		expectedStatus int
	}{
		{
			name:           "self-deactivation",
			payload:        map[string]any{"is_active": false},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "self-demotion",
			payload:        map[string]any{"role": "user"},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			admin := &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "admin-id"},
					TenantID: "tenant-a",
				},
				Email:    "admin@example.com",
				Name:     "Admin User",
				Role:     models.UserRoleAdmin,
				IsActive: true,
			}
			reg := &mockUserRegistryForSecurityTests{
				users: map[string]*models.User{"admin-id": admin},
			}
			r := newUsersRouter(admin, reg)

			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPut, "/users/admin-id", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			c.Assert(w.Code, qt.Equals, tt.expectedStatus, qt.Commentf("test: %s", tt.name))
		})
	}
}
