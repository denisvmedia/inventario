package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/debug"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
	"github.com/denisvmedia/inventario/services"
)

// setupTestAPIServer creates a test API server with authentication
func setupTestAPIServer(t *testing.T) (*httptest.Server, *models.User, *models.User, string, func()) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN environment variable not set")
		return nil, nil, nil, "", nil
	}

	registrySetFunc, cleanupFunc := postgres.NewPostgresRegistrySet()
	registrySet, err := registrySetFunc(registry.Config(dsn))
	if err != nil {
		t.Fatalf("Failed to create registry set: %v", err)
	}

	jwtSecret := []byte("test-secret-32-bytes-minimum-length")

	// Create test users
	user1 := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "api-user-1"},
			TenantID: "test-tenant-id",
		},
		Email:    "user1@api-test.com",
		Name:     "API Test User 1",
		Role:     models.UserRoleUser,
		IsActive: true,
	}
	err = user1.SetPassword("testpassword123")
	if err != nil {
		t.Fatalf("Failed to set password: %v", err)
	}

	user2 := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "api-user-2"},
			TenantID: "test-tenant-id",
		},
		Email:    "user2@api-test.com",
		Name:     "API Test User 2",
		Role:     models.UserRoleUser,
		IsActive: true,
	}
	err = user2.SetPassword("testpassword123")
	if err != nil {
		t.Fatalf("Failed to set password: %v", err)
	}

	createdUser1, err := registrySet.UserRegistry.Create(context.Background(), user1)
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}

	createdUser2, err := registrySet.UserRegistry.Create(context.Background(), user2)
	if err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}

	// Create API server
	params := apiserver.Params{
		RegistrySet:    registrySet,
		EntityService:  services.NewEntityService(registrySet, "file://uploads?memfs=1&create_dir=1"),
		UploadLocation: "file://uploads?memfs=1&create_dir=1",
		DebugInfo:      debug.NewInfo("postgres://test", "file://uploads?memfs=1&create_dir=1"),
		StartTime:      time.Now(),
		JWTSecret:      jwtSecret,
	}

	handler := apiserver.APIServer(params, nil)
	server := httptest.NewServer(handler)

	cleanup := func() {
		server.Close()
		if cleanupFunc != nil {
			cleanupFunc()
		}
	}

	return server, createdUser1, createdUser2, string(jwtSecret), cleanup
}

// generateJWTToken creates a JWT token for the given user
func generateJWTToken(user *models.User, jwtSecret string) (string, error) {
	expiresAt := time.Now().Add(24 * time.Hour)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"role":    user.Role,
		"exp":     expiresAt.Unix(),
	})

	return token.SignedString([]byte(jwtSecret))
}

// makeAuthenticatedRequest makes an HTTP request with JWT authentication
func makeAuthenticatedRequest(method, url string, body []byte, token string) (*http.Response, error) {
	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequest(method, url, bytes.NewBuffer(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	return client.Do(req)
}

// TestAPIUserIsolation_Commodities tests commodity API isolation
func TestAPIUserIsolation_Commodities(t *testing.T) {
	server, user1, user2, jwtSecret, cleanup := setupTestAPIServer(t)
	defer cleanup()

	// Generate tokens for both users
	token1, err := generateJWTToken(user1, jwtSecret)
	if err != nil {
		t.Fatalf("Failed to generate token for user1: %v", err)
	}

	token2, err := generateJWTToken(user2, jwtSecret)
	if err != nil {
		t.Fatalf("Failed to generate token for user2: %v", err)
	}

	// User1 creates a commodity
	commodityData := map[string]interface{}{
		"name":        "User1 Commodity",
		"description": "A commodity created by user1",
	}
	jsonData, err := json.Marshal(commodityData)
	if err != nil {
		t.Fatalf("Failed to marshal commodity data: %v", err)
	}

	resp, err := makeAuthenticatedRequest("POST", server.URL+"/api/v1/commodities", jsonData, token1)
	if err != nil {
		t.Fatalf("Failed to create commodity: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}
	resp.Body.Close()

	// User2 tries to list commodities - should see empty list
	resp, err = makeAuthenticatedRequest("GET", server.URL+"/api/v1/commodities", nil, token2)
	if err != nil {
		t.Fatalf("Failed to list commodities for user2: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var commodities []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&commodities)
	if err != nil {
		t.Fatalf("Failed to decode commodities: %v", err)
	}
	if len(commodities) != 0 {
		t.Errorf("Expected 0 commodities for user2, got %d", len(commodities))
	}
	resp.Body.Close()

	// User1 can see their commodity
	resp, err = makeAuthenticatedRequest("GET", server.URL+"/api/v1/commodities", nil, token1)
	if err != nil {
		t.Fatalf("Failed to list commodities for user1: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	err = json.NewDecoder(resp.Body).Decode(&commodities)
	if err != nil {
		t.Fatalf("Failed to decode commodities for user1: %v", err)
	}
	if len(commodities) != 1 {
		t.Errorf("Expected 1 commodity for user1, got %d", len(commodities))
	}
	if commodities[0]["name"] != "User1 Commodity" {
		t.Errorf("Expected 'User1 Commodity', got %v", commodities[0]["name"])
	}
	resp.Body.Close()
}

// TestAPIAuthentication tests authentication requirements for API endpoints
func TestAPIAuthentication(t *testing.T) {
	server, _, _, _, cleanup := setupTestAPIServer(t)
	defer cleanup()

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/commodities"},
		{"POST", "/api/v1/commodities"},
		{"GET", "/api/v1/locations"},
		{"POST", "/api/v1/locations"},
		{"GET", "/api/v1/areas"},
		{"POST", "/api/v1/areas"},
		{"GET", "/api/v1/files"},
		{"GET", "/api/v1/exports"},
		{"POST", "/api/v1/exports"},
	}

	for _, endpoint := range endpoints {
		t.Run(fmt.Sprintf("%s %s", endpoint.method, endpoint.path), func(t *testing.T) {
			// Test without authentication - should fail
			var req *http.Request
			var err error

			if endpoint.method == "POST" {
				testData := map[string]interface{}{"name": "Test"}
				jsonData, _ := json.Marshal(testData)
				req, err = http.NewRequest(endpoint.method, server.URL+endpoint.path, bytes.NewBuffer(jsonData))
			} else {
				req, err = http.NewRequest(endpoint.method, server.URL+endpoint.path, nil)
			}

			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
			}
			resp.Body.Close()
		})
	}
}
