//go:build integration

package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
)

func setupTestDatabase(t *testing.T) (registry.RegistrySet, func()) {
	// Use environment variable for database DSN
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN environment variable not set")
	}

	registrySet, err := postgres.NewRegistrySet(dsn)
	if err != nil {
		t.Fatalf("Failed to create registry set: %v", err)
	}

	// Cleanup function
	cleanup := func() {
		// Clean up test data
		ctx := context.Background()
		users, _ := registrySet.UserRegistry.List(ctx, registry.Filter{})
		for _, user := range users {
			registrySet.UserRegistry.Delete(ctx, user.ID)
		}
	}

	return registrySet, cleanup
}

func TestAuthIntegration_FullAuthenticationFlow(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestDatabase(t)
	defer cleanup()

	jwtSecret := []byte("test-secret-32-bytes-minimum-length")

	// Create test user in database
	testUser := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "integration-user-123"},
			TenantID: "test-tenant-id",
		},
		Email:    "integration@example.com",
		Name:     "Integration Test User",
		Role:     models.UserRoleUser,
		IsActive: true,
	}
	err := testUser.SetPassword("IntegrationTest123")
	c.Assert(err, qt.IsNil)

	createdUser, err := registrySet.UserRegistry.Create(context.Background(), testUser)
	c.Assert(err, qt.IsNil)
	c.Assert(createdUser, qt.IsNotNil)

	// Create auth handler
	authHandler := apiserver.Auth(apiserver.AuthParams{UserRegistry: registrySet.UserRegistry, JWTSecret: jwtSecret})
	r := chi.NewRouter()
	r.Route("/auth", authHandler)

	t.Run("successful login flow", func(t *testing.T) {
		c := qt.New(t)

		// Step 1: Login with valid credentials
		loginReq := map[string]string{
			"email":    "integration@example.com",
			"password": "IntegrationTest123",
		}
		reqBody, _ := json.Marshal(loginReq)

		req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		c.Assert(w.Code, qt.Equals, http.StatusOK)

		// Parse response
		var loginResp struct {
			AccessToken string       `json:"access_token"`
			User        *models.User `json:"user"`
			ExpiresIn   int          `json:"expires_in"`
		}
		err := json.NewDecoder(w.Body).Decode(&loginResp)
		c.Assert(err, qt.IsNil)
		c.Assert(loginResp.AccessToken, qt.Not(qt.Equals), "")
		c.Assert(loginResp.User.Email, qt.Equals, "integration@example.com")

		// Step 2: Use token to access protected endpoint
		middleware := apiserver.JWTMiddleware(jwtSecret, registrySet.UserRegistry, nil)
		protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := apiserver.UserFromContext(r.Context())
			c.Assert(user, qt.IsNotNil)
			c.Assert(user.Email, qt.Equals, "integration@example.com")
			w.WriteHeader(http.StatusOK)
		})

		protectedReq := httptest.NewRequest("GET", "/protected", nil)
		protectedReq.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
		protectedW := httptest.NewRecorder()

		middleware(protectedHandler).ServeHTTP(protectedW, protectedReq)
		c.Assert(protectedW.Code, qt.Equals, http.StatusOK)

		// Step 3: Test logout
		logoutReq := httptest.NewRequest("POST", "/auth/logout", nil)
		logoutW := httptest.NewRecorder()

		r.ServeHTTP(logoutW, logoutReq)
		c.Assert(logoutW.Code, qt.Equals, http.StatusOK)

		// Note: Token should still be valid since logout is client-side only
		// This is a known limitation documented in the security analysis
	})

	t.Run("failed login attempts", func(t *testing.T) {
		c := qt.New(t)

		tests := []struct {
			name     string
			email    string
			password string
			expected int
		}{
			{
				name:     "wrong password",
				email:    "integration@example.com",
				password: "WrongPassword123",
				expected: http.StatusUnauthorized,
			},
			{
				name:     "non-existent user",
				email:    "nonexistent@example.com",
				password: "IntegrationTest123",
				expected: http.StatusUnauthorized,
			},
			{
				name:     "empty credentials",
				email:    "",
				password: "",
				expected: http.StatusBadRequest,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				c := qt.New(t)

				loginReq := map[string]string{
					"email":    tt.email,
					"password": tt.password,
				}
				reqBody, _ := json.Marshal(loginReq)

				req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(reqBody))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)
				c.Assert(w.Code, qt.Equals, tt.expected)
			})
		}
	})
}

func TestAuthIntegration_UserStatusChanges(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestDatabase(t)
	defer cleanup()

	jwtSecret := []byte("test-secret-32-bytes-minimum-length")

	// Create test user
	testUser := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "status-test-user"},
			TenantID: "test-tenant-id",
		},
		Email:    "status@example.com",
		Name:     "Status Test User",
		Role:     models.UserRoleUser,
		IsActive: true,
	}
	err := testUser.SetPassword("StatusTest123")
	c.Assert(err, qt.IsNil)

	createdUser, err := registrySet.UserRegistry.Create(context.Background(), testUser)
	c.Assert(err, qt.IsNil)

	// Login to get token
	authHandler := apiserver.Auth(apiserver.AuthParams{UserRegistry: registrySet.UserRegistry, JWTSecret: jwtSecret})
	r := chi.NewRouter()
	r.Route("/auth", authHandler)

	loginReq := map[string]string{
		"email":    "status@example.com",
		"password": "StatusTest123",
	}
	reqBody, _ := json.Marshal(loginReq)

	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	var loginResp struct {
		AccessToken string `json:"access_token"`
	}
	err = json.NewDecoder(w.Body).Decode(&loginResp)
	c.Assert(err, qt.IsNil)

	// Test access with active user
	middleware := apiserver.JWTMiddleware(jwtSecret, registrySet.UserRegistry, nil)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	protectedReq := httptest.NewRequest("GET", "/protected", nil)
	protectedReq.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	protectedW := httptest.NewRecorder()

	middleware(handler).ServeHTTP(protectedW, protectedReq)
	c.Assert(protectedW.Code, qt.Equals, http.StatusOK)

	// Deactivate user
	createdUser.IsActive = false
	_, err = registrySet.UserRegistry.Update(context.Background(), *createdUser)
	c.Assert(err, qt.IsNil)

	// Test access with deactivated user - should be denied
	protectedReq2 := httptest.NewRequest("GET", "/protected", nil)
	protectedReq2.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	protectedW2 := httptest.NewRecorder()

	middleware(handler).ServeHTTP(protectedW2, protectedReq2)
	c.Assert(protectedW2.Code, qt.Equals, http.StatusForbidden)
}

func TestAuthIntegration_DatabaseConnectivity(t *testing.T) {
	c := qt.New(t)

	// Test with invalid DSN
	_, err := postgres.NewRegistrySet("invalid://dsn")
	c.Assert(err, qt.IsNotNil, qt.Commentf("Should fail with invalid DSN"))

	// Test with valid DSN
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		c.Skip("POSTGRES_TEST_DSN environment variable not set")
		return
	}

	registrySet, err := postgres.NewRegistrySet(dsn)
	c.Assert(err, qt.IsNil, qt.Commentf("Should succeed with valid DSN"))
	c.Assert(registrySet, qt.IsNotNil)
	c.Assert(registrySet.UserRegistry, qt.IsNotNil)
}

func TestAuthIntegration_ConcurrentUserOperations(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestDatabase(t)
	defer cleanup()

	jwtSecret := []byte("test-secret-32-bytes-minimum-length")

	// Create multiple users concurrently
	const numUsers = 5
	userChan := make(chan *models.User, numUsers)
	errorChan := make(chan error, numUsers)

	for i := 0; i < numUsers; i++ {
		go func(index int) {
			testUser := models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "concurrent-user-" + string(rune('0'+index))},
					TenantID: "test-tenant-id",
				},
				Email:    "concurrent" + string(rune('0'+index)) + "@example.com",
				Name:     "Concurrent User " + string(rune('0'+index)),
				Role:     models.UserRoleUser,
				IsActive: true,
			}
			err := testUser.SetPassword("ConcurrentTest123")
			if err != nil {
				errorChan <- err
				return
			}

			createdUser, err := registrySet.UserRegistry.Create(context.Background(), testUser)
			if err != nil {
				errorChan <- err
				return
			}

			userChan <- createdUser
		}(i)
	}

	// Collect results
	var createdUsers []*models.User
	for i := 0; i < numUsers; i++ {
		select {
		case user := <-userChan:
			createdUsers = append(createdUsers, user)
		case err := <-errorChan:
			c.Fatalf("Error creating user: %v", err)
		case <-time.After(10 * time.Second):
			c.Fatalf("Timeout waiting for user creation")
		}
	}

	c.Assert(len(createdUsers), qt.Equals, numUsers)

	// Test concurrent authentication
	authHandler := apiserver.Auth(apiserver.AuthParams{UserRegistry: registrySet.UserRegistry, JWTSecret: jwtSecret})
	r := chi.NewRouter()
	r.Route("/auth", authHandler)

	tokenChan := make(chan string, numUsers)
	authErrorChan := make(chan error, numUsers)

	for i, user := range createdUsers {
		go func(index int, u *models.User) {
			loginReq := map[string]string{
				"email":    u.Email,
				"password": "ConcurrentTest123",
			}
			reqBody, _ := json.Marshal(loginReq)

			req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				authErrorChan <- qt.Errorf("Login failed for user %s: status %d", u.Email, w.Code)
				return
			}

			var loginResp struct {
				AccessToken string `json:"access_token"`
			}
			err := json.NewDecoder(w.Body).Decode(&loginResp)
			if err != nil {
				authErrorChan <- err
				return
			}

			tokenChan <- loginResp.AccessToken
		}(i, user)
	}

	// Collect authentication results
	var tokens []string
	for i := 0; i < numUsers; i++ {
		select {
		case token := <-tokenChan:
			tokens = append(tokens, token)
		case err := <-authErrorChan:
			c.Fatalf("Authentication error: %v", err)
		case <-time.After(10 * time.Second):
			c.Fatalf("Timeout waiting for authentication")
		}
	}

	c.Assert(len(tokens), qt.Equals, numUsers)
	c.Assert(len(tokens), qt.Equals, len(createdUsers))
}
