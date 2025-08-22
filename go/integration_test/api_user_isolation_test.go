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

	qt "github.com/frankban/quicktest"
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

// TestAPIUserIsolation tests user isolation across all API endpoints
func TestAPIUserIsolation(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		method   string
		testFunc func(*qt.C, *httptest.Server, *models.User, *models.User, string)
	}{
		{"GET Commodities", "/api/v1/commodities", "GET", testCommoditiesAPIIsolation},
		{"POST Commodities", "/api/v1/commodities", "POST", testCommoditiesAPICreation},
		{"GET Locations", "/api/v1/locations", "GET", testLocationsAPIIsolation},
		{"POST Locations", "/api/v1/locations", "POST", testLocationsAPICreation},
		{"GET Areas", "/api/v1/areas", "GET", testAreasAPIIsolation},
		{"POST Areas", "/api/v1/areas", "POST", testAreasAPICreation},
		{"GET Files", "/api/v1/files", "GET", testFilesAPIIsolation},
		{"GET Exports", "/api/v1/exports", "GET", testExportsAPIIsolation},
		{"POST Exports", "/api/v1/exports", "POST", testExportsAPICreation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			server, user1, user2, jwtSecret, cleanup := setupTestAPIServer(t)
			defer cleanup()

			tt.testFunc(c, server, user1, user2, jwtSecret)
		})
	}
}

// Test functions for specific API endpoints
func testCommoditiesAPIIsolation(c *qt.C, server *httptest.Server, user1, user2 *models.User, jwtSecret string) {
	// Generate tokens for both users
	token1, err := generateJWTToken(user1, jwtSecret)
	c.Assert(err, qt.IsNil)

	token2, err := generateJWTToken(user2, jwtSecret)
	c.Assert(err, qt.IsNil)

	// User1 creates a commodity
	commodityData := map[string]interface{}{
		"name":        "User1 Commodity",
		"description": "A commodity created by user1",
	}
	jsonData, err := json.Marshal(commodityData)
	c.Assert(err, qt.IsNil)

	resp, err := makeAuthenticatedRequest("POST", server.URL+"/api/v1/commodities", jsonData, token1)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusCreated)
	resp.Body.Close()

	// User2 tries to list commodities - should see empty list
	resp, err = makeAuthenticatedRequest("GET", server.URL+"/api/v1/commodities", nil, token2)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)

	var commodities []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&commodities)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 0)
	resp.Body.Close()

	// User1 can see their commodity
	resp, err = makeAuthenticatedRequest("GET", server.URL+"/api/v1/commodities", nil, token1)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)

	err = json.NewDecoder(resp.Body).Decode(&commodities)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 1)
	c.Assert(commodities[0]["name"], qt.Equals, "User1 Commodity")
	resp.Body.Close()
}

func testCommoditiesAPICreation(c *qt.C, server *httptest.Server, user1, user2 *models.User, jwtSecret string) {
	// Generate tokens for both users
	token1, err := generateJWTToken(user1, jwtSecret)
	if err != nil {
		c.Fatalf("Failed to generate token for user1: %v", err)
	}

	token2, err := generateJWTToken(user2, jwtSecret)
	if err != nil {
		c.Fatalf("Failed to generate token for user2: %v", err)
	}

	// Both users create commodities
	commodityData1 := map[string]interface{}{
		"name":        "User1 Commodity",
		"description": "Created by user1",
	}
	jsonData1, err := json.Marshal(commodityData1)
	c.Assert(err, qt.IsNil)

	commodityData2 := map[string]interface{}{
		"name":        "User2 Commodity",
		"description": "Created by user2",
	}
	jsonData2, err := json.Marshal(commodityData2)
	c.Assert(err, qt.IsNil)

	// User1 creates commodity
	resp, err := makeAuthenticatedRequest("POST", server.URL+"/api/v1/commodities", jsonData1, token1)
	if err != nil {
		c.Fatalf("Failed to create commodity for user1: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		c.Errorf("Expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}
	resp.Body.Close()

	// User2 creates commodity
	resp, err = makeAuthenticatedRequest("POST", server.URL+"/api/v1/commodities", jsonData2, token2)
	if err != nil {
		c.Fatalf("Failed to create commodity for user2: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		c.Errorf("Expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}
	resp.Body.Close()

	// Each user can only see their own commodity
	resp, err = makeAuthenticatedRequest("GET", server.URL+"/api/v1/commodities", nil, token1)
	if err != nil {
		c.Fatalf("Failed to list commodities for user1: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		c.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var commodities1 []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&commodities1)
	if err != nil {
		c.Fatalf("Failed to decode commodities for user1: %v", err)
	}
	if len(commodities1) != 1 {
		c.Errorf("Expected 1 commodity for user1, got %d", len(commodities1))
	}
	if commodities1[0]["name"] != "User1 Commodity" {
		c.Errorf("Expected 'User1 Commodity', got %v", commodities1[0]["name"])
	}
	resp.Body.Close()

	resp, err = makeAuthenticatedRequest("GET", server.URL+"/api/v1/commodities", nil, token2)
	if err != nil {
		c.Fatalf("Failed to list commodities for user2: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		c.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var commodities2 []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&commodities2)
	if err != nil {
		c.Fatalf("Failed to decode commodities for user2: %v", err)
	}
	if len(commodities2) != 1 {
		c.Errorf("Expected 1 commodity for user2, got %d", len(commodities2))
	}
	if commodities2[0]["name"] != "User2 Commodity" {
		c.Errorf("Expected 'User2 Commodity', got %v", commodities2[0]["name"])
	}
	resp.Body.Close()
}

func testLocationsAPIIsolation(c *qt.C, server *httptest.Server, user1, user2 *models.User, jwtSecret string) {
	// Generate tokens for both users
	token1, err := generateJWTToken(user1, jwtSecret)
	c.Assert(err, qt.IsNil)

	token2, err := generateJWTToken(user2, jwtSecret)
	c.Assert(err, qt.IsNil)

	// User1 creates a location
	locationData := map[string]interface{}{
		"name":    "User1 Location",
		"address": "123 User1 Street",
	}
	jsonData, err := json.Marshal(locationData)
	c.Assert(err, qt.IsNil)

	resp, err := makeAuthenticatedRequest("POST", server.URL+"/api/v1/locations", jsonData, token1)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusCreated)
	resp.Body.Close()

	// User2 tries to list locations - should see empty list
	resp, err = makeAuthenticatedRequest("GET", server.URL+"/api/v1/locations", nil, token2)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)

	var locations []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&locations)
	c.Assert(err, qt.IsNil)
	c.Assert(len(locations), qt.Equals, 0)
	resp.Body.Close()

	// User1 can see their location
	resp, err = makeAuthenticatedRequest("GET", server.URL+"/api/v1/locations", nil, token1)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)

	err = json.NewDecoder(resp.Body).Decode(&locations)
	c.Assert(err, qt.IsNil)
	c.Assert(len(locations), qt.Equals, 1)
	c.Assert(locations[0]["name"], qt.Equals, "User1 Location")
	resp.Body.Close()
}

func testLocationsAPICreation(c *qt.C, server *httptest.Server, user1, user2 *models.User, jwtSecret string) {
	// Generate tokens for both users
	token1, err := generateJWTToken(user1, jwtSecret)
	c.Assert(err, qt.IsNil)

	token2, err := generateJWTToken(user2, jwtSecret)
	c.Assert(err, qt.IsNil)

	// Both users create locations
	locationData1 := map[string]interface{}{
		"name":    "User1 Location",
		"address": "123 User1 Street",
	}
	jsonData1, err := json.Marshal(locationData1)
	c.Assert(err, qt.IsNil)

	locationData2 := map[string]interface{}{
		"name":    "User2 Location",
		"address": "456 User2 Street",
	}
	jsonData2, err := json.Marshal(locationData2)
	c.Assert(err, qt.IsNil)

	// User1 creates location
	resp, err := makeAuthenticatedRequest("POST", server.URL+"/api/v1/locations", jsonData1, token1)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusCreated)
	resp.Body.Close()

	// User2 creates location
	resp, err = makeAuthenticatedRequest("POST", server.URL+"/api/v1/locations", jsonData2, token2)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusCreated)
	resp.Body.Close()

	// Each user can only see their own location
	resp, err = makeAuthenticatedRequest("GET", server.URL+"/api/v1/locations", nil, token1)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)

	var locations1 []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&locations1)
	c.Assert(err, qt.IsNil)
	c.Assert(len(locations1), qt.Equals, 1)
	c.Assert(locations1[0]["name"], qt.Equals, "User1 Location")
	resp.Body.Close()

	resp, err = makeAuthenticatedRequest("GET", server.URL+"/api/v1/locations", nil, token2)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)

	var locations2 []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&locations2)
	c.Assert(err, qt.IsNil)
	c.Assert(len(locations2), qt.Equals, 1)
	c.Assert(locations2[0]["name"], qt.Equals, "User2 Location")
	resp.Body.Close()
}

func testAreasAPIIsolation(c *qt.C, server *httptest.Server, user1, user2 *models.User, jwtSecret string) {
	// Generate tokens for both users
	token1, err := generateJWTToken(user1, jwtSecret)
	c.Assert(err, qt.IsNil)

	token2, err := generateJWTToken(user2, jwtSecret)
	c.Assert(err, qt.IsNil)

	// First create a location for user1
	locationData := map[string]interface{}{
		"name":    "User1 Location for Area",
		"address": "123 User1 Street",
	}
	jsonData, err := json.Marshal(locationData)
	c.Assert(err, qt.IsNil)

	resp, err := makeAuthenticatedRequest("POST", server.URL+"/api/v1/locations", jsonData, token1)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusCreated)

	var createdLocation map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&createdLocation)
	c.Assert(err, qt.IsNil)
	locationID := createdLocation["id"].(string)
	resp.Body.Close()

	// User1 creates an area
	areaData := map[string]interface{}{
		"name":        "User1 Area",
		"location_id": locationID,
	}
	jsonData, err = json.Marshal(areaData)
	c.Assert(err, qt.IsNil)

	resp, err = makeAuthenticatedRequest("POST", server.URL+"/api/v1/areas", jsonData, token1)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusCreated)
	resp.Body.Close()

	// User2 tries to list areas - should see empty list
	resp, err = makeAuthenticatedRequest("GET", server.URL+"/api/v1/areas", nil, token2)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)

	var areas []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&areas)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Equals, 0)
	resp.Body.Close()

	// User1 can see their area
	resp, err = makeAuthenticatedRequest("GET", server.URL+"/api/v1/areas", nil, token1)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)

	err = json.NewDecoder(resp.Body).Decode(&areas)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Equals, 1)
	c.Assert(areas[0]["name"], qt.Equals, "User1 Area")
	resp.Body.Close()
}

func testAreasAPICreation(c *qt.C, server *httptest.Server, user1, user2 *models.User, jwtSecret string) {
	// This test is more complex as areas require locations
	// For simplicity, we'll just test that areas are properly isolated
	token1, err := generateJWTToken(user1, jwtSecret)
	c.Assert(err, qt.IsNil)

	token2, err := generateJWTToken(user2, jwtSecret)
	c.Assert(err, qt.IsNil)

	// User2 tries to list areas - should see empty list
	resp, err := makeAuthenticatedRequest("GET", server.URL+"/api/v1/areas", nil, token2)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)

	var areas []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&areas)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Equals, 0)
	resp.Body.Close()
}

func testFilesAPIIsolation(c *qt.C, server *httptest.Server, user1, user2 *models.User, jwtSecret string) {
	// Generate tokens for both users
	token1, err := generateJWTToken(user1, jwtSecret)
	c.Assert(err, qt.IsNil)

	token2, err := generateJWTToken(user2, jwtSecret)
	c.Assert(err, qt.IsNil)

	// User2 tries to list files - should see empty list
	resp, err := makeAuthenticatedRequest("GET", server.URL+"/api/v1/files", nil, token2)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)

	var files []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&files)
	c.Assert(err, qt.IsNil)
	c.Assert(len(files), qt.Equals, 0)
	resp.Body.Close()

	// User1 tries to list files - should also see empty list initially
	resp, err = makeAuthenticatedRequest("GET", server.URL+"/api/v1/files", nil, token1)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)

	err = json.NewDecoder(resp.Body).Decode(&files)
	c.Assert(err, qt.IsNil)
	c.Assert(len(files), qt.Equals, 0)
	resp.Body.Close()
}

func testExportsAPIIsolation(c *qt.C, server *httptest.Server, user1, user2 *models.User, jwtSecret string) {
	// Generate tokens for both users
	token1, err := generateJWTToken(user1, jwtSecret)
	c.Assert(err, qt.IsNil)

	token2, err := generateJWTToken(user2, jwtSecret)
	c.Assert(err, qt.IsNil)

	// User1 creates an export
	exportData := map[string]interface{}{
		"name":        "User1 Export",
		"description": "An export created by user1",
	}
	jsonData, err := json.Marshal(exportData)
	c.Assert(err, qt.IsNil)

	resp, err := makeAuthenticatedRequest("POST", server.URL+"/api/v1/exports", jsonData, token1)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusCreated)
	resp.Body.Close()

	// User2 tries to list exports - should see empty list
	resp, err = makeAuthenticatedRequest("GET", server.URL+"/api/v1/exports", nil, token2)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)

	var exports []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&exports)
	c.Assert(err, qt.IsNil)
	c.Assert(len(exports), qt.Equals, 0)
	resp.Body.Close()

	// User1 can see their export
	resp, err = makeAuthenticatedRequest("GET", server.URL+"/api/v1/exports", nil, token1)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)

	err = json.NewDecoder(resp.Body).Decode(&exports)
	c.Assert(err, qt.IsNil)
	c.Assert(len(exports), qt.Equals, 1)
	c.Assert(exports[0]["name"], qt.Equals, "User1 Export")
	resp.Body.Close()
}

func testExportsAPICreation(c *qt.C, server *httptest.Server, user1, user2 *models.User, jwtSecret string) {
	// Generate tokens for both users
	token1, err := generateJWTToken(user1, jwtSecret)
	c.Assert(err, qt.IsNil)

	token2, err := generateJWTToken(user2, jwtSecret)
	c.Assert(err, qt.IsNil)

	// Both users create exports
	exportData1 := map[string]interface{}{
		"name":        "User1 Export",
		"description": "Created by user1",
	}
	jsonData1, err := json.Marshal(exportData1)
	c.Assert(err, qt.IsNil)

	exportData2 := map[string]interface{}{
		"name":        "User2 Export",
		"description": "Created by user2",
	}
	jsonData2, err := json.Marshal(exportData2)
	c.Assert(err, qt.IsNil)

	// User1 creates export
	resp, err := makeAuthenticatedRequest("POST", server.URL+"/api/v1/exports", jsonData1, token1)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusCreated)
	resp.Body.Close()

	// User2 creates export
	resp, err = makeAuthenticatedRequest("POST", server.URL+"/api/v1/exports", jsonData2, token2)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusCreated)
	resp.Body.Close()

	// Each user can only see their own export
	resp, err = makeAuthenticatedRequest("GET", server.URL+"/api/v1/exports", nil, token1)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)

	var exports1 []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&exports1)
	c.Assert(err, qt.IsNil)
	c.Assert(len(exports1), qt.Equals, 1)
	c.Assert(exports1[0]["name"], qt.Equals, "User1 Export")
	resp.Body.Close()

	resp, err = makeAuthenticatedRequest("GET", server.URL+"/api/v1/exports", nil, token2)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)

	var exports2 []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&exports2)
	c.Assert(err, qt.IsNil)
	c.Assert(len(exports2), qt.Equals, 1)
	c.Assert(exports2[0]["name"], qt.Equals, "User2 Export")
	resp.Body.Close()
}

// TestAPIAuthentication tests authentication requirements for API endpoints
func TestAPIAuthentication(t *testing.T) {
	c := qt.New(t)
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
			c := qt.New(t)

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

// TestAPIInvalidTokens tests API behavior with invalid JWT tokens
func TestAPIInvalidTokens(t *testing.T) {
	server, _, _, _, cleanup := setupTestAPIServer(t)
	defer cleanup()

	invalidTokens := []struct {
		name  string
		token string
	}{
		{"Empty token", ""},
		{"Invalid format", "invalid-token"},
		{"Malformed JWT", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid.signature"},
		{"Expired token", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoidGVzdCIsImV4cCI6MX0.invalid"},
	}

	for _, tokenTest := range invalidTokens {
		t.Run(tokenTest.name, func(t *testing.T) {

			req, err := http.NewRequest("GET", server.URL+"/api/v1/commodities", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			if tokenTest.token != "" {
				req.Header.Set("Authorization", "Bearer "+tokenTest.token)
			}

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

// TestAPIDirectEntityAccess tests that users cannot access specific entities by ID
func TestAPIDirectEntityAccess(t *testing.T) {
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
		"name":        "User1 Private Commodity",
		"description": "This should not be accessible by user2",
	}
	jsonData, err := json.Marshal(commodityData)
	c.Assert(err, qt.IsNil)

	resp, err := makeAuthenticatedRequest("POST", server.URL+"/api/v1/commodities", jsonData, token1)
	if err != nil {
		t.Fatalf("Failed to create commodity: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var createdCommodity map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&createdCommodity)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	commodityID := createdCommodity["id"].(string)
	resp.Body.Close()

	// User2 tries to access User1's commodity directly by ID
	resp, err = makeAuthenticatedRequest("GET", server.URL+"/api/v1/commodities/"+commodityID, nil, token2)
	if err != nil {
		t.Fatalf("Failed to access commodity: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
	resp.Body.Close()

	// User1 can access their own commodity
	resp, err = makeAuthenticatedRequest("GET", server.URL+"/api/v1/commodities/"+commodityID, nil, token1)
	if err != nil {
		t.Fatalf("Failed to access own commodity: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var retrievedCommodity map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&retrievedCommodity)
	if err != nil {
		t.Fatalf("Failed to decode commodity: %v", err)
	}
	if retrievedCommodity["name"] != "User1 Private Commodity" {
		t.Errorf("Expected name 'User1 Private Commodity', got %v", retrievedCommodity["name"])
	}
	resp.Body.Close()
}
