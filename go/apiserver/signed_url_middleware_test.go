package apiserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// mockUserRegistry implements registry.UserRegistry for testing
type mockUserRegistry struct {
	users map[string]*models.User
}

func newMockUserRegistry() *mockUserRegistry {
	return &mockUserRegistry{
		users: make(map[string]*models.User),
	}
}

func (m *mockUserRegistry) addUser(user *models.User) {
	m.users[user.ID] = user
}

func (m *mockUserRegistry) Get(ctx context.Context, id string) (*models.User, error) {
	user, exists := m.users[id]
	if !exists {
		return nil, registry.ErrNotFound
	}
	return user, nil
}

// Implement other required methods (not used in tests)
func (m *mockUserRegistry) Create(ctx context.Context, user models.User) (*models.User, error) {
	return nil, registry.ErrNotFound
}

func (m *mockUserRegistry) Update(ctx context.Context, user models.User) (*models.User, error) {
	return nil, registry.ErrNotFound
}

func (m *mockUserRegistry) Delete(ctx context.Context, id string) error {
	return registry.ErrNotFound
}

func (m *mockUserRegistry) List(ctx context.Context) ([]*models.User, error) {
	return nil, registry.ErrNotFound
}

func (m *mockUserRegistry) Count(ctx context.Context) (int, error) {
	return 0, registry.ErrNotFound
}

func (m *mockUserRegistry) GetByEmail(ctx context.Context, tenantID, email string) (*models.User, error) {
	return nil, registry.ErrNotFound
}

func (m *mockUserRegistry) ListByTenant(ctx context.Context, tenantID string) ([]*models.User, error) {
	return nil, registry.ErrNotFound
}

func (m *mockUserRegistry) ListByRole(ctx context.Context, tenantID string, role models.UserRole) ([]*models.User, error) {
	return nil, registry.ErrNotFound
}

func TestSignedURLMiddleware(t *testing.T) {
	c := qt.New(t)

	// Setup
	signingKey := []byte("test-signing-key-32-bytes-long!!")
	expiration := 15 * time.Minute
	fileSigningService := services.NewFileSigningService(signingKey, expiration)
	userRegistry := newMockUserRegistry()

	// Create test user
	testUser := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-123"},
			TenantID: "test-tenant-123",
			UserID:   "test-user-123",
		},
		Email:    "test@example.com",
		Name:     "Test User",
		IsActive: true,
		Role:     models.UserRoleUser,
	}
	userRegistry.addUser(testUser)

	// Create middleware
	middleware := apiserver.SignedURLMiddleware(fileSigningService, userRegistry)

	// Create a test handler that checks if user context is set
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := appctx.UserFromContext(r.Context())
		if user == nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("no user in context"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	wrappedHandler := middleware(testHandler)

	tests := []struct {
		name           string
		method         string
		path           string
		query          string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "non-GET request",
			method:         "POST",
			path:           "/api/v1/files/download/test.pdf",
			query:          "",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method not allowed for signed URLs\n",
		},
		{
			name:           "missing signature",
			method:         "GET",
			path:           "/api/v1/files/download/test.pdf",
			query:          "exp=9999999999&uid=test-user-123",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid or expired file URL\n",
		},
		{
			name:           "missing expiration",
			method:         "GET",
			path:           "/api/v1/files/download/test.pdf",
			query:          "sig=invalid&uid=test-user-123",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid or expired file URL\n",
		},
		{
			name:           "missing user ID",
			method:         "GET",
			path:           "/api/v1/files/download/test.pdf",
			query:          "sig=invalid&exp=9999999999",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid or expired file URL\n",
		},
		{
			name:           "invalid signature",
			method:         "GET",
			path:           "/api/v1/files/download/test.pdf",
			query:          "sig=invalid&exp=9999999999&uid=test-user-123",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid or expired file URL\n",
		},
		{
			name:           "user not found (missing signature)",
			method:         "GET",
			path:           "/api/v1/files/download/test.pdf",
			query:          "",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid or expired file URL\n",
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			// Create request
			req := httptest.NewRequest(tt.method, tt.path+"?"+tt.query, nil)
			w := httptest.NewRecorder()

			// Execute middleware
			wrappedHandler.ServeHTTP(w, req)

			// Check response
			c.Assert(w.Code, qt.Equals, tt.expectedStatus)
			c.Assert(w.Body.String(), qt.Equals, tt.expectedBody)
		})
	}

	c.Run("valid signed URL", func(c *qt.C) {
		// Generate a valid signed URL
		signedURL, err := fileSigningService.GenerateSignedURL("test-file", "pdf", testUser.ID)
		c.Assert(err, qt.IsNil)

		// Parse the URL
		parsedURL, err := url.Parse(signedURL)
		c.Assert(err, qt.IsNil)

		// Create request with valid signed URL
		req := httptest.NewRequest("GET", parsedURL.Path+"?"+parsedURL.RawQuery, nil)
		w := httptest.NewRecorder()

		// Execute middleware
		wrappedHandler.ServeHTTP(w, req)

		// Check response
		c.Assert(w.Code, qt.Equals, http.StatusOK)
		c.Assert(w.Body.String(), qt.Equals, "success")
	})

	c.Run("inactive user", func(c *qt.C) {
		// Create inactive user
		inactiveUser := &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "inactive-user-123"},
				TenantID: "test-tenant-123",
				UserID:   "inactive-user-123",
			},
			Email:    "inactive@example.com",
			Name:     "Inactive User",
			IsActive: false,
			Role:     models.UserRoleUser,
		}
		userRegistry.addUser(inactiveUser)

		// Generate signed URL for inactive user
		signedURL, err := fileSigningService.GenerateSignedURL("test-file", "pdf", inactiveUser.ID)
		c.Assert(err, qt.IsNil)

		// Parse the URL
		parsedURL, err := url.Parse(signedURL)
		c.Assert(err, qt.IsNil)

		// Create request
		req := httptest.NewRequest("GET", parsedURL.Path+"?"+parsedURL.RawQuery, nil)
		w := httptest.NewRecorder()

		// Execute middleware
		wrappedHandler.ServeHTTP(w, req)

		// Check response
		c.Assert(w.Code, qt.Equals, http.StatusForbidden)
		c.Assert(w.Body.String(), qt.Equals, "User account disabled\n")
	})

	c.Run("expired URL", func(c *qt.C) {
		// Create service with very short expiration
		shortExpirationService := services.NewFileSigningService(signingKey, 1*time.Nanosecond)

		// Generate signed URL that will expire immediately
		signedURL, err := shortExpirationService.GenerateSignedURL("test-file", "pdf", testUser.ID)
		c.Assert(err, qt.IsNil)

		// Wait for expiration
		time.Sleep(10 * time.Millisecond)

		// Parse the URL
		parsedURL, err := url.Parse(signedURL)
		c.Assert(err, qt.IsNil)

		// Create request
		req := httptest.NewRequest("GET", parsedURL.Path+"?"+parsedURL.RawQuery, nil)
		w := httptest.NewRecorder()

		// Execute middleware
		wrappedHandler.ServeHTTP(w, req)

		// Check response
		c.Assert(w.Code, qt.Equals, http.StatusUnauthorized)
		c.Assert(w.Body.String(), qt.Equals, "Invalid or expired file URL\n")
	})

	c.Run("user not found with valid signature", func(c *qt.C) {
		// Generate signed URL for non-existent user
		signedURL, err := fileSigningService.GenerateSignedURL("test-file", "pdf", "non-existent-user")
		c.Assert(err, qt.IsNil)

		// Parse the URL
		parsedURL, err := url.Parse(signedURL)
		c.Assert(err, qt.IsNil)

		// Create request
		req := httptest.NewRequest("GET", parsedURL.Path+"?"+parsedURL.RawQuery, nil)
		w := httptest.NewRecorder()

		// Execute middleware
		wrappedHandler.ServeHTTP(w, req)

		// Should fail with user not found
		c.Assert(w.Code, qt.Equals, http.StatusUnauthorized)
		c.Assert(w.Body.String(), qt.Equals, "User not found\n")
	})
}

func TestSignedURLMiddleware_SecurityScenarios(t *testing.T) {
	c := qt.New(t)

	// Setup
	signingKey := []byte("test-signing-key-32-bytes-long!!")
	expiration := 15 * time.Minute
	fileSigningService := services.NewFileSigningService(signingKey, expiration)
	userRegistry := newMockUserRegistry()

	// Create test users
	user1 := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "test-tenant-123",
			UserID:   "user-1",
		},
		Email:    "user1@example.com",
		Name:     "User 1",
		IsActive: true,
		Role:     models.UserRoleUser,
	}
	user2 := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-2"},
			TenantID: "test-tenant-123",
			UserID:   "user-2",
		},
		Email:    "user2@example.com",
		Name:     "User 2",
		IsActive: true,
		Role:     models.UserRoleUser,
	}
	userRegistry.addUser(user1)
	userRegistry.addUser(user2)

	middleware := apiserver.SignedURLMiddleware(fileSigningService, userRegistry)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := appctx.UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(user.ID))
	})

	wrappedHandler := middleware(testHandler)

	c.Run("user cannot access file with another user's signature", func(c *qt.C) {
		// Generate signed URL for user1
		signedURL, err := fileSigningService.GenerateSignedURL("test-file", "pdf", user1.ID)
		c.Assert(err, qt.IsNil)

		// Parse and modify the URL to use user2's ID
		parsedURL, err := url.Parse(signedURL)
		c.Assert(err, qt.IsNil)

		query := parsedURL.Query()
		query.Set("uid", user2.ID) // Try to access with different user ID
		parsedURL.RawQuery = query.Encode()

		// Create request
		req := httptest.NewRequest("GET", parsedURL.Path+"?"+parsedURL.RawQuery, nil)
		w := httptest.NewRecorder()

		// Execute middleware
		wrappedHandler.ServeHTTP(w, req)

		// Should fail due to signature mismatch
		c.Assert(w.Code, qt.Equals, http.StatusUnauthorized)
		c.Assert(w.Body.String(), qt.Equals, "Invalid or expired file URL\n")
	})

	c.Run("tampering with file ID invalidates signature", func(c *qt.C) {
		// Generate signed URL for a file
		signedURL, err := fileSigningService.GenerateSignedURL("original-file", "pdf", user1.ID)
		c.Assert(err, qt.IsNil)

		// Parse the URL
		parsedURL, err := url.Parse(signedURL)
		c.Assert(err, qt.IsNil)

		// Modify the path to access a different file
		parsedURL.Path = "/api/v1/files/download/different-file.pdf"

		// Create request
		req := httptest.NewRequest("GET", parsedURL.Path+"?"+parsedURL.RawQuery, nil)
		w := httptest.NewRecorder()

		// Execute middleware
		wrappedHandler.ServeHTTP(w, req)

		// Should fail due to signature mismatch
		c.Assert(w.Code, qt.Equals, http.StatusUnauthorized)
		c.Assert(w.Body.String(), qt.Equals, "Invalid or expired file URL\n")
	})
}
