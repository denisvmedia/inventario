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

func (m *mockUserRegistryForUsersTests) Update(ctx context.Context, user models.User) (*models.User, error) {
	for _, u := range m.users {
		if u.ID != user.ID && u.Email == user.Email && u.TenantID == user.TenantID {
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
// RequireAdmin blocks all access until group-based authorization is added
// -----------------------------------------------------------------------

func TestUsersAPI_RequireAdmin_BlocksAllAccess(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   map[string]any
	}{
		{
			name:   "list users",
			method: http.MethodGet,
			path:   "/users",
		},
		{
			name:   "get user",
			method: http.MethodGet,
			path:   "/users/target-id",
		},
		{
			name:   "create user",
			method: http.MethodPost,
			path:   "/users",
			body: map[string]any{
				"email":    "new-user@example.com",
				"password": "ValidPass123!",
				"name":     "New User",
			},
		},
		{
			name:   "update user",
			method: http.MethodPut,
			path:   "/users/target-id",
			body:   map[string]any{"name": "Updated"},
		},
		{
			name:   "deactivate user",
			method: http.MethodDelete,
			path:   "/users/target-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			caller := &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "caller-id"},
					TenantID: "tenant-a",
				},
				Email:    "caller@example.com",
				Name:     "Caller User",
				IsActive: true,
			}
			reg := &mockUserRegistryForUsersTests{
				mockUserRegistryForSecurityTests: mockUserRegistryForSecurityTests{
					users: map[string]*models.User{"caller-id": caller},
				},
			}
			r := newUsersRouter(caller, reg)

			var body []byte
			if tt.body != nil {
				body, _ = json.Marshal(tt.body)
			}
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(body))
			if tt.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			c.Assert(w.Code, qt.Equals, http.StatusForbidden,
				qt.Commentf("RequireAdmin should block %s %s until group-based auth is implemented", tt.method, tt.path))
		})
	}
}

func TestUsersAPI_RequireAdmin_Unauthenticated_Returns401(t *testing.T) {
	c := qt.New(t)

	reg := &mockUserRegistryForUsersTests{
		mockUserRegistryForSecurityTests: mockUserRegistryForSecurityTests{
			users: map[string]*models.User{},
		},
	}

	r := chi.NewRouter()
	// Do NOT inject a user into context — simulate an unauthenticated request.
	r.Route("/users", apiserver.Users(apiserver.UsersParams{UserRegistry: reg}))

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusUnauthorized)
}
