package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/golang-jwt/jwt/v5"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/debug"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
	"github.com/denisvmedia/inventario/services"
)

// setupTestAPIServer creates a test API server with authentication
func setupTestAPIServer(t *testing.T) (server *httptest.Server, user1 *models.User, user2 *models.User, jwtSecret string, registrySet *registry.Set, cleanup func()) {
	c := qt.New(t)
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN environment variable not set")
		return nil, nil, nil, "", nil, nil
	}

	// Set up fresh database with bootstrap and migrations
	err := setupFreshDatabase(dsn)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to setup fresh database"))

	registrySetFunc, cleanupFunc := postgres.NewPostgresRegistrySet()
	factorySet, err := registrySetFunc(registry.Config(dsn))
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create factory set"))

	jwtSecretBytes := []byte("test-secret-32-bytes-minimum-length")

	// Create test tenant first with unique identifiers
	testTenant := models.Tenant{
		Name:   "Test Tenant API " + time.Now().Format("20060102-150405"),
		Slug:   "test-tenant-api-" + fmt.Sprintf("%d", time.Now().UnixNano()%1000000),
		Status: models.TenantStatusActive,
	}
	createdTenant, err := factorySet.TenantRegistry.Create(context.Background(), testTenant)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create test tenant"))
	testTenantID := createdTenant.ID

	// Create test users with unique identifiers
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
	user1Model := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: testTenantID,
		},
		Email:    "user1-" + timestamp + "@api-test.com",
		Name:     "API Test User 1",
		Role:     models.UserRoleUser,
		IsActive: true,
	}
	err = user1Model.SetPassword("testpassword123")
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to set password for user1"))

	user2Model := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: testTenantID,
		},
		Email:    "user2-" + timestamp + "@api-test.com",
		Name:     "API Test User 2",
		Role:     models.UserRoleUser,
		IsActive: true,
	}
	err = user2Model.SetPassword("testpassword123")
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to set password for user2"))

	createdUser1, err := factorySet.UserRegistry.Create(context.Background(), user1Model)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user1"))

	createdUser2, err := factorySet.UserRegistry.Create(context.Background(), user2Model)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user2"))

	// Create user-aware registry set for user1 (for testing purposes)
	ctx := appctx.WithUser(context.Background(), createdUser1)
	registrySet, err = factorySet.CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user registry set"))

	// Create API server
	params := apiserver.Params{
		FactorySet:     factorySet,
		EntityService:  services.NewEntityService(factorySet, "file://uploads?memfs=1&create_dir=1"),
		UploadLocation: "file://uploads?memfs=1&create_dir=1",
		DebugInfo:      debug.NewInfo("postgres://test", "file://uploads?memfs=1&create_dir=1"),
		StartTime:      time.Now(),
		JWTSecret:      jwtSecretBytes,
	}

	handler := apiserver.APIServer(params, nil)
	server = httptest.NewServer(handler)

	cleanup = func() {
		server.Close()
		if cleanupFunc != nil {
			cleanupFunc()
		}
	}

	return server, createdUser1, createdUser2, string(jwtSecretBytes), registrySet, cleanup
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
	server, createdUser1, createdUser2, jwtSecret, registrySet, cleanup := setupTestAPIServer(t)
	defer cleanup()

	// Get the variables we need
	testTenantID := createdUser1.TenantID // Both users have the same tenant ID

	// Skip main currency setup for now - we'll create a commodity without price validation
	// This avoids the RLS issues with settings

	err := registrySet.SettingsRegistry.Save(context.Background(), models.SettingsObject{
		MainCurrency: new("USD"),
	})
	if err != nil {
		t.Fatalf("Failed to save settings: %v", err)
	}

	// Create a location and area for the commodity
	location := models.Location{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: testTenantID,
			UserID:   createdUser1.ID,
		},
		Name:    "Test Location",
		Address: "123 Test St",
	}

	// Create location using user1's context
	ctx := appctx.WithUser(context.Background(), createdUser1)
	createdLocation, err := registrySet.LocationRegistry.Create(ctx, location)
	if err != nil {
		t.Fatalf("Failed to create location: %v", err)
	}

	area := models.Area{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: testTenantID,
			UserID:   createdUser1.ID,
		},
		Name:       "Test Area",
		LocationID: createdLocation.ID,
	}
	createdArea, err := registrySet.AreaRegistry.Create(ctx, area)
	if err != nil {
		t.Fatalf("Failed to create area: %v", err)
	}

	// Generate tokens for both users
	token1, err := generateJWTToken(createdUser1, jwtSecret)
	if err != nil {
		t.Fatalf("Failed to generate token for user1: %v", err)
	}

	token2, err := generateJWTToken(createdUser2, jwtSecret)
	if err != nil {
		t.Fatalf("Failed to generate token for user2: %v", err)
	}

	// User1 creates a commodity (as draft to avoid price validation)
	//commodityData := map[string]any{
	//	"name":        "User1 Commodity",
	//	"description": "A commodity created by user1",
	//	"area_id":     area.ID,
	//	"count":       1,
	//	"status":      "in_use",
	//	"draft":       true, // Create as draft to bypass main currency validation
	//}

	obj := &jsonapi.CommodityRequest{
		Data: &jsonapi.CommodityData{
			Type: "commodities",
			Attributes: &models.Commodity{
				Name:                   "New Commodity in Area 2",
				ShortName:              "NewCom2",
				AreaID:                 createdArea.ID,
				Type:                   models.CommodityTypeElectronics,
				OriginalPrice:          must.Must(decimal.NewFromString("1000.00")),
				OriginalPriceCurrency:  models.Currency("USD"),
				ConvertedOriginalPrice: must.Must(decimal.NewFromString("0")), // to pass the validation
				CurrentPrice:           must.Must(decimal.NewFromString("800.00")),
				SerialNumber:           "SN123456",
				ExtraSerialNumbers:     []string{"SN654321"},
				PartNumbers:            []string{"P123", "P456"},
				Tags:                   []string{"tag1", "tag2"},
				Count:                  1,
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
				RegisteredDate:         models.ToPDate("2023-01-02"),
				LastModifiedDate:       models.ToPDate("2023-01-03"),
				Comments:               "New commodity comments",
				Draft:                  true,
			},
		},
	}

	jsonData, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("Failed to marshal commodity data: %v", err)
	}

	resp, err := makeAuthenticatedRequest("POST", server.URL+"/api/v1/commodities", jsonData, token1)
	if err != nil {
		t.Fatalf("Failed to create commodity: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status %d, got %d. Response body: %s", http.StatusCreated, resp.StatusCode, string(body))
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

	var commodities map[string]any
	err = json.NewDecoder(resp.Body).Decode(&commodities)
	if err != nil {
		t.Fatalf("Failed to decode commodities: %v", err)
	}
	if len(commodities["data"].([]any)) != 0 {
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

	commodities = map[string]any(nil)
	err = json.NewDecoder(resp.Body).Decode(&commodities)
	if err != nil {
		t.Fatalf("Failed to decode commodities for user1: %v", err)
	}
	dataArray := commodities["data"].([]any)
	if len(dataArray) != 1 {
		t.Errorf("Expected 1 commodity for user1, got %d", len(dataArray))
	}
	if len(dataArray) > 0 {
		name := dataArray[0].(map[string]any)["attributes"].(map[string]any)["name"]
		if name != obj.Data.Attributes.Name {
			t.Errorf("Expected '%s', got %v", obj.Data.Attributes.Name, name)
		}
	}
	resp.Body.Close()
}

// TestAPIAuthentication tests authentication requirements for API endpoints
func TestAPIAuthentication(t *testing.T) {
	server, _, _, _, _, cleanup := setupTestAPIServer(t)
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
				testData := map[string]any{"name": "Test"}
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
