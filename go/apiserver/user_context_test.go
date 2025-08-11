package apiserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

func TestJWTUserResolver_ResolveUser(t *testing.T) {
	jwtSecret := []byte("test-secret")

	// Helper function to create a valid JWT token
	createToken := func(userID string) string {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id":   userID,
			"tenant_id": "tenant-123",
			"role":      "user",
			"exp":       time.Now().Add(24 * time.Hour).Unix(),
		})
		tokenString, _ := token.SignedString(jwtSecret)
		return tokenString
	}

	// Happy path tests
	t.Run("resolve user from valid JWT token", func(t *testing.T) {
		c := qt.New(t)
		resolver := apiserver.NewJWTUserResolver(jwtSecret)

		req := httptest.NewRequest("GET", "/", nil)
		token := createToken("user-123")
		req.Header.Set("Authorization", "Bearer "+token)

		userID, err := resolver.ResolveUser(req)
		c.Assert(err, qt.IsNil)
		c.Assert(userID, qt.Equals, "user-123")
	})

	// Unhappy path tests
	t.Run("missing authorization header", func(t *testing.T) {
		c := qt.New(t)
		resolver := apiserver.NewJWTUserResolver(jwtSecret)

		req := httptest.NewRequest("GET", "/", nil)

		userID, err := resolver.ResolveUser(req)
		c.Assert(err, qt.IsNotNil)
		c.Assert(userID, qt.Equals, "")
		c.Assert(err, qt.Equals, apiserver.ErrUserNotFound)
	})

	t.Run("empty authorization header", func(t *testing.T) {
		c := qt.New(t)
		resolver := apiserver.NewJWTUserResolver(jwtSecret)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "")

		userID, err := resolver.ResolveUser(req)
		c.Assert(err, qt.IsNotNil)
		c.Assert(userID, qt.Equals, "")
		c.Assert(err, qt.Equals, apiserver.ErrUserNotFound)
	})

	t.Run("invalid bearer token format", func(t *testing.T) {
		c := qt.New(t)
		resolver := apiserver.NewJWTUserResolver(jwtSecret)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "InvalidFormat token")

		userID, err := resolver.ResolveUser(req)
		c.Assert(err, qt.IsNotNil)
		c.Assert(userID, qt.Equals, "")
		c.Assert(err, qt.Equals, apiserver.ErrUserNotFound)
	})

	t.Run("invalid JWT token", func(t *testing.T) {
		c := qt.New(t)
		resolver := apiserver.NewJWTUserResolver(jwtSecret)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer invalid.jwt.token")

		userID, err := resolver.ResolveUser(req)
		c.Assert(err, qt.IsNotNil)
		c.Assert(userID, qt.Equals, "")
		c.Assert(err, qt.Equals, apiserver.ErrUserNotFound)
	})

	t.Run("JWT token with wrong secret", func(t *testing.T) {
		c := qt.New(t)
		resolver := apiserver.NewJWTUserResolver(jwtSecret)

		// Create token with different secret
		wrongSecret := []byte("wrong-secret")
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id":   "user-123",
			"tenant_id": "tenant-123",
			"role":      "user",
			"exp":       time.Now().Add(24 * time.Hour).Unix(),
		})
		tokenString, _ := token.SignedString(wrongSecret)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)

		userID, err := resolver.ResolveUser(req)
		c.Assert(err, qt.IsNotNil)
		c.Assert(userID, qt.Equals, "")
		c.Assert(err, qt.Equals, apiserver.ErrUserNotFound)
	})

	t.Run("JWT token without user_id claim", func(t *testing.T) {
		c := qt.New(t)
		resolver := apiserver.NewJWTUserResolver(jwtSecret)

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"tenant_id": "tenant-123",
			"role":      "user",
			"exp":       time.Now().Add(24 * time.Hour).Unix(),
		})
		tokenString, _ := token.SignedString(jwtSecret)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)

		userID, err := resolver.ResolveUser(req)
		c.Assert(err, qt.IsNotNil)
		c.Assert(userID, qt.Equals, "")
		c.Assert(err, qt.Equals, apiserver.ErrUserNotFound)
	})

	t.Run("expired JWT token", func(t *testing.T) {
		c := qt.New(t)
		resolver := apiserver.NewJWTUserResolver(jwtSecret)

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id":   "user-123",
			"tenant_id": "tenant-123",
			"role":      "user",
			"exp":       time.Now().Add(-1 * time.Hour).Unix(), // Expired 1 hour ago
		})
		tokenString, _ := token.SignedString(jwtSecret)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)

		userID, err := resolver.ResolveUser(req)
		c.Assert(err, qt.IsNotNil)
		c.Assert(userID, qt.Equals, "")
		c.Assert(err, qt.Equals, apiserver.ErrUserNotFound)
	})
}

func TestUserFromContext(t *testing.T) {
	// Happy path tests
	t.Run("retrieve user from context", func(t *testing.T) {
		c := qt.New(t)
		user := &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "user-123"},
				TenantID: "tenant-123",
			},
			Email:    "test@example.com",
			Name:     "Test User",
			Role:     models.UserRoleUser,
			IsActive: true,
		}

		ctx := apiserver.WithUser(context.Background(), user)
		retrievedUser := apiserver.UserFromContext(ctx)

		c.Assert(retrievedUser, qt.IsNotNil)
		c.Assert(retrievedUser.ID, qt.Equals, "user-123")
		c.Assert(retrievedUser.Email, qt.Equals, "test@example.com")
		c.Assert(retrievedUser.Name, qt.Equals, "Test User")
	})

	// Unhappy path tests
	t.Run("no user in context", func(t *testing.T) {
		c := qt.New(t)
		retrievedUser := apiserver.UserFromContext(context.Background())
		c.Assert(retrievedUser, qt.IsNil)
	})
}

func TestUserIDFromContext(t *testing.T) {
	// Happy path tests
	t.Run("retrieve user ID from context", func(t *testing.T) {
		c := qt.New(t)
		ctx := apiserver.WithUserID(context.Background(), "user-123")
		userID := apiserver.UserIDFromContext(ctx)

		c.Assert(userID, qt.Equals, "user-123")
	})

	t.Run("retrieve user ID from user context", func(t *testing.T) {
		c := qt.New(t)
		user := &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "user-456"},
				TenantID: "tenant-123",
			},
			Email:    "test@example.com",
			Name:     "Test User",
			Role:     models.UserRoleUser,
			IsActive: true,
		}

		ctx := apiserver.WithUser(context.Background(), user)
		userID := apiserver.UserIDFromContext(ctx)

		c.Assert(userID, qt.Equals, "user-456")
	})

	// Unhappy path tests
	t.Run("no user ID in context", func(t *testing.T) {
		c := qt.New(t)
		userID := apiserver.UserIDFromContext(context.Background())
		c.Assert(userID, qt.Equals, "")
	})
}

func TestWithUser(t *testing.T) {
	t.Run("add user to context", func(t *testing.T) {
		c := qt.New(t)
		user := &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "user-123"},
				TenantID: "tenant-123",
			},
			Email:    "test@example.com",
			Name:     "Test User",
			Role:     models.UserRoleUser,
			IsActive: true,
		}

		ctx := apiserver.WithUser(context.Background(), user)

		retrievedUser := apiserver.UserFromContext(ctx)
		c.Assert(retrievedUser, qt.IsNotNil)
		c.Assert(retrievedUser.ID, qt.Equals, "user-123")

		userID := apiserver.UserIDFromContext(ctx)
		c.Assert(userID, qt.Equals, "user-123")
	})
}

func TestWithUserID(t *testing.T) {
	t.Run("add user ID to context", func(t *testing.T) {
		c := qt.New(t)
		ctx := apiserver.WithUserID(context.Background(), "user-123")

		userID := apiserver.UserIDFromContext(ctx)
		c.Assert(userID, qt.Equals, "user-123")

		// User object should not be available
		user := apiserver.UserFromContext(ctx)
		c.Assert(user, qt.IsNil)
	})
}

// Mock user registry for testing middleware
type mockUserRegistry struct {
	users map[string]*models.User
}

func (m *mockUserRegistry) Get(ctx context.Context, id string) (*models.User, error) {
	user, exists := m.users[id]
	if !exists {
		return nil, registry.ErrNotFound
	}
	return user, nil
}

func (m *mockUserRegistry) Create(ctx context.Context, user models.User) (*models.User, error) {
	return nil, nil
}

func (m *mockUserRegistry) Update(ctx context.Context, user models.User) (*models.User, error) {
	return nil, nil
}

func (m *mockUserRegistry) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockUserRegistry) List(ctx context.Context) ([]*models.User, error) {
	return nil, nil
}

func (m *mockUserRegistry) Count(ctx context.Context) (int, error) {
	return 0, nil
}

func (m *mockUserRegistry) GetByEmail(ctx context.Context, tenantID, email string) (*models.User, error) {
	return nil, nil
}

func (m *mockUserRegistry) ListByTenant(ctx context.Context, tenantID string) ([]*models.User, error) {
	return nil, nil
}

func (m *mockUserRegistry) ListByRole(ctx context.Context, tenantID string, role models.UserRole) ([]*models.User, error) {
	return nil, nil
}

func TestUserMiddleware(t *testing.T) {
	jwtSecret := []byte("test-secret")

	// Create test user
	testUser := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-123"},
			TenantID: "tenant-123",
		},
		Email:    "test@example.com",
		Name:     "Test User",
		Role:     models.UserRoleUser,
		IsActive: true,
	}

	// Create mock registry
	userRegistry := &mockUserRegistry{
		users: map[string]*models.User{
			"user-123": testUser,
		},
	}

	// Create resolver
	resolver := apiserver.NewJWTUserResolver(jwtSecret)

	// Helper function to create a valid JWT token
	createToken := func(userID string) string {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id":   userID,
			"tenant_id": "tenant-123",
			"role":      "user",
			"exp":       time.Now().Add(24 * time.Hour).Unix(),
		})
		tokenString, _ := token.SignedString(jwtSecret)
		return tokenString
	}

	// Happy path tests
	t.Run("successful user middleware with valid token and active user", func(t *testing.T) {
		c := qt.New(t)

		middleware := apiserver.UserMiddleware(resolver, userRegistry)

		// Create a test handler that checks if user is in context
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := apiserver.UserFromContext(r.Context())
			c.Assert(user, qt.IsNotNil)
			c.Assert(user.ID, qt.Equals, "user-123")
			c.Assert(user.Email, qt.Equals, "test@example.com")
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/", nil)
		token := createToken("user-123")
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		middleware(handler).ServeHTTP(rr, req)

		c.Assert(rr.Code, qt.Equals, http.StatusOK)
	})

	// Unhappy path tests
	t.Run("user middleware with invalid token", func(t *testing.T) {
		c := qt.New(t)

		middleware := apiserver.UserMiddleware(resolver, userRegistry)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Errorf("Handler should not be called")
		})

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer invalid.token")

		rr := httptest.NewRecorder()
		middleware(handler).ServeHTTP(rr, req)

		c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
	})

	t.Run("user middleware with non-existent user", func(t *testing.T) {
		c := qt.New(t)

		middleware := apiserver.UserMiddleware(resolver, userRegistry)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Errorf("Handler should not be called")
		})

		req := httptest.NewRequest("GET", "/", nil)
		token := createToken("non-existent-user")
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		middleware(handler).ServeHTTP(rr, req)

		c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
	})

	t.Run("user middleware with inactive user", func(t *testing.T) {
		c := qt.New(t)

		// Create inactive user
		inactiveUser := &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "inactive-user"},
				TenantID: "tenant-123",
			},
			Email:    "inactive@example.com",
			Name:     "Inactive User",
			Role:     models.UserRoleUser,
			IsActive: false,
		}

		userRegistryWithInactive := &mockUserRegistry{
			users: map[string]*models.User{
				"user-123":      testUser,
				"inactive-user": inactiveUser,
			},
		}

		middleware := apiserver.UserMiddleware(resolver, userRegistryWithInactive)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Errorf("Handler should not be called")
		})

		req := httptest.NewRequest("GET", "/", nil)
		token := createToken("inactive-user")
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		middleware(handler).ServeHTTP(rr, req)

		c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
	})
}

func TestRequireUser(t *testing.T) {
	// Happy path tests
	t.Run("require user middleware with user in context", func(t *testing.T) {
		c := qt.New(t)

		middleware := apiserver.RequireUser()

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		testUser := &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "user-123"},
				TenantID: "tenant-123",
			},
			Email:    "test@example.com",
			Name:     "Test User",
			Role:     models.UserRoleUser,
			IsActive: true,
		}

		req := httptest.NewRequest("GET", "/", nil)
		ctx := apiserver.WithUser(req.Context(), testUser)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		middleware(handler).ServeHTTP(rr, req)

		c.Assert(rr.Code, qt.Equals, http.StatusOK)
	})

	// Unhappy path tests
	t.Run("require user middleware without user in context", func(t *testing.T) {
		c := qt.New(t)

		middleware := apiserver.RequireUser()

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Errorf("Handler should not be called")
		})

		req := httptest.NewRequest("GET", "/", nil)

		rr := httptest.NewRecorder()
		middleware(handler).ServeHTTP(rr, req)

		c.Assert(rr.Code, qt.Equals, http.StatusInternalServerError)
	})
}

func TestUserAwareMiddleware(t *testing.T) {
	jwtSecret := []byte("test-secret")
	resolver := apiserver.NewJWTUserResolver(jwtSecret)

	// Helper function to create a valid JWT token
	createToken := func(userID string) string {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id":   userID,
			"tenant_id": "tenant-123",
			"role":      "user",
			"exp":       time.Now().Add(24 * time.Hour).Unix(),
		})
		tokenString, _ := token.SignedString(jwtSecret)
		return tokenString
	}

	// Happy path tests
	t.Run("user aware middleware with valid token", func(t *testing.T) {
		c := qt.New(t)

		middleware := apiserver.UserAwareMiddleware(resolver)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := apiserver.UserIDFromContext(r.Context())
			c.Assert(userID, qt.Equals, "user-123")
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/", nil)
		token := createToken("user-123")
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		middleware(handler).ServeHTTP(rr, req)

		c.Assert(rr.Code, qt.Equals, http.StatusOK)
	})

	t.Run("user aware middleware without token continues processing", func(t *testing.T) {
		c := qt.New(t)

		middleware := apiserver.UserAwareMiddleware(resolver)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := apiserver.UserIDFromContext(r.Context())
			c.Assert(userID, qt.Equals, "")
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/", nil)

		rr := httptest.NewRecorder()
		middleware(handler).ServeHTTP(rr, req)

		c.Assert(rr.Code, qt.Equals, http.StatusOK)
	})

	t.Run("user aware middleware with invalid token continues processing", func(t *testing.T) {
		c := qt.New(t)

		middleware := apiserver.UserAwareMiddleware(resolver)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := apiserver.UserIDFromContext(r.Context())
			c.Assert(userID, qt.Equals, "")
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer invalid.token")

		rr := httptest.NewRecorder()
		middleware(handler).ServeHTTP(rr, req)

		c.Assert(rr.Code, qt.Equals, http.StatusOK)
	})
}
