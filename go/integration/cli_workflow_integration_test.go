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
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	usercreate "github.com/denisvmedia/inventario/cmd/inventario/users/create"
	"github.com/denisvmedia/inventario/debug"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
	"github.com/denisvmedia/inventario/services"
)

// TestCLIWorkflowIntegration tests the complete workflow from fresh database setup
// through CLI operations to API access, simulating a CI pipeline scenario
func TestCLIWorkflowIntegration(t *testing.T) {
	c := qt.New(t)

	// Register PostgreSQL registry for CLI commands
	postgres.Register()

	// Get PostgreSQL DSN or skip test
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN environment variable not set")
		return
	}

	// Step 1: Setup fresh database with bootstrap and migrations
	t.Log("🔧 Setting up fresh database with bootstrap and migrations...")
	err := setupFreshDatabase(dsn)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to setup fresh database"))

	// Step 2: Attempt login with non-existent user (should fail)
	t.Log("🔐 Testing login with non-existent user (should fail)...")
	server, cleanup := setupAPIServer(t, dsn)
	defer cleanup()

	loginSuccess := attemptLogin(t, server.URL, "nonexistent@example.com", "password123")
	c.Assert(loginSuccess, qt.IsFalse, qt.Commentf("Login should fail for non-existent user"))

	// Step 3: Create tenant with specific ID to match API server expectations
	// Note: The API server uses hardcoded defaultTenantID = "test-tenant-id" for user lookup
	// We need to create a tenant with this exact ID for the integration test to work
	t.Log("🏢 Creating tenant with specific ID for API server compatibility...")
	tenantID := "test-tenant-id"
	tenantSlug := "test-company"
	err = createTenantWithSpecificID(dsn, tenantID, "Test Company", tenantSlug, "test-company.com")
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create tenant with specific ID"))

	// Debug: Check what tenant was actually created
	t.Logf("Created tenant with ID: %s, slug: %s", tenantID, tenantSlug)

	// Step 4: Create user via CLI
	t.Log("👤 Creating user via CLI...")
	userEmail := "admin@test-company.com"
	userPassword := "SecurePassword123!"
	err = createUserViaCLI(dsn, userEmail, userPassword, "Admin User", tenantSlug, "admin")
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user via CLI"))

	// Step 4.5: List all tenants to debug the mismatch
	t.Log("🔍 Listing all tenants in database...")
	err = listAllTenants(dsn)
	if err != nil {
		t.Logf("⚠️  Could not list tenants: %v", err)
	}

	// Step 4.6: Verify user was actually created in the database
	t.Log("🔍 Verifying user was created in database...")
	err = verifyUserExists(dsn, tenantID, userEmail)
	c.Assert(err, qt.IsNil, qt.Commentf("User was not found in database after CLI creation"))

	// Step 5: Attempt login with created user (should succeed)
	t.Log("🔐 Testing login with created user (should succeed)...")
	loginSuccess = attemptLogin(t, server.URL, userEmail, userPassword)
	c.Assert(loginSuccess, qt.IsTrue, qt.Commentf("Login should succeed for created user"))

	// Step 6: Get authentication token
	t.Log("🎫 Getting authentication token...")
	token, err := getAuthToken(server.URL, userEmail, userPassword)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to get authentication token"))
	c.Assert(token, qt.Not(qt.Equals), "", qt.Commentf("Token should not be empty"))

	// Step 7: Access system info API with valid token (should succeed)
	t.Log("📊 Testing system info API access with valid token...")
	systemInfo, err := getSystemInfo(server.URL, token)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to access system info API"))
	c.Assert(systemInfo, qt.IsNotNil, qt.Commentf("System info should not be nil"))
	c.Assert(systemInfo["database_backend"], qt.Equals, "postgres", qt.Commentf("Database backend should be postgres"))

	t.Log("✅ CLI workflow integration test completed successfully!")
}

// createTenantWithSpecificID creates a tenant with a specific ID directly in the database
// This is needed for integration tests where the API server expects a specific tenant ID
func createTenantWithSpecificID(dsn, tenantID, name, slug, domain string) error {
	// Create registry set
	registrySetFunc, cleanupFunc := postgres.NewPostgresRegistrySet()
	defer func() {
		if cleanupFunc != nil {
			cleanupFunc()
		}
	}()

	factorySet, err := registrySetFunc(registry.Config(dsn))
	if err != nil {
		return fmt.Errorf("failed to create factory set: %w", err)
	}

	// Create tenant with specific ID
	tenant := models.Tenant{
		EntityID: models.EntityID{ID: tenantID}, // Set specific ID
		Name:     name,
		Slug:     slug,
		Status:   models.TenantStatusActive,
	}
	if domain != "" {
		tenant.Domain = &domain
	}

	// Insert directly using the registry
	_, err = factorySet.TenantRegistry.Create(context.Background(), tenant)
	if err != nil {
		return fmt.Errorf("failed to create tenant with specific ID: %w", err)
	}

	return nil
}

// listAllTenants lists all tenants in the database for debugging
func listAllTenants(dsn string) error {
	// Create registry set to query the database
	registrySetFunc, cleanupFunc := postgres.NewPostgresRegistrySet()
	defer cleanupFunc()

	factorySet, err := registrySetFunc(registry.Config(dsn))
	if err != nil {
		return fmt.Errorf("failed to create factory set: %w", err)
	}

	// Get all tenants
	tenants, err := factorySet.TenantRegistry.List(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list tenants: %w", err)
	}

	fmt.Printf("📋 Found %d tenants in database:\n", len(tenants))
	for i, tenant := range tenants {
		fmt.Printf("  %d. ID=%s, Slug=%s, Name=%s, Status=%s\n",
			i+1, tenant.ID, tenant.Slug, tenant.Name, tenant.Status)
	}

	return nil
}

// getTenantIDFromCLIOutput finds the actual tenant ID by querying for the user
func getTenantIDFromCLIOutput(dsn, email string) (string, error) {
	// Create registry set to query the database
	registrySetFunc, cleanupFunc := postgres.NewPostgresRegistrySet()
	defer cleanupFunc()

	factorySet, err := registrySetFunc(registry.Config(dsn))
	if err != nil {
		return "", fmt.Errorf("failed to create factory set: %w", err)
	}

	// Get all users and find the one with matching email
	users, err := factorySet.UserRegistry.List(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to list users: %w", err)
	}

	for _, user := range users {
		if user.Email == email {
			return user.TenantID, nil
		}
	}

	return "", fmt.Errorf("user with email %s not found", email)
}

// verifyUserExists checks if a user exists in the database
func verifyUserExists(dsn, tenantID, email string) error {
	// Create registry set to query the database
	registrySetFunc, cleanupFunc := postgres.NewPostgresRegistrySet()
	defer cleanupFunc()

	factorySet, err := registrySetFunc(registry.Config(dsn))
	if err != nil {
		return fmt.Errorf("failed to create factory set: %w", err)
	}

	// Try to get the user
	user, err := factorySet.UserRegistry.GetByEmail(context.Background(), tenantID, email)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Log user details for debugging
	fmt.Printf("✅ User found: ID=%s, Email=%s, TenantID=%s, Name=%s, Role=%s\n",
		user.ID, user.Email, user.TenantID, user.Name, user.Role)

	return nil
}

// createUserViaCLI creates a user using the CLI command
func createUserViaCLI(dsn, email, password, name, tenant, role string) error {
	var dbConfig shared.DatabaseConfig

	// Create root command with database flags
	rootCmd := &cobra.Command{
		Use: "inventario",
	}
	shared.RegisterDatabaseFlags(rootCmd, &dbConfig)

	// Add user subcommand
	userCmd := usercreate.New(&dbConfig)
	rootCmd.AddCommand(userCmd.Cmd())

	// Capture output
	var output bytes.Buffer
	rootCmd.SetOut(&output)
	rootCmd.SetErr(&output)

	// Set arguments for the full command
	args := []string{
		"--db-dsn=" + dsn,
		"create",
		"--email=" + email,
		"--password=" + password,
		"--name=" + name,
		"--tenant=" + tenant,
		"--role=" + role,
		"--no-interactive",
	}

	// Debug: Print the arguments being passed
	fmt.Printf("🔍 CLI user creation args: %v\n", args)

	rootCmd.SetArgs(args)

	if err := rootCmd.Execute(); err != nil {
		return fmt.Errorf("user creation failed: %w\nOutput: %s", err, output.String())
	}

	// Debug: Print the output to see what actually happened
	fmt.Printf("🔍 CLI user creation output: %s\n", output.String())

	return nil
}

// setupAPIServer creates a test API server for testing authentication and API access
func setupAPIServer(t *testing.T, dsn string) (*httptest.Server, func()) {
	// Create registry set
	registrySetFunc, cleanupFunc := postgres.NewPostgresRegistrySet()
	factorySet, err := registrySetFunc(registry.Config(dsn))
	if err != nil {
		t.Fatalf("Failed to create factory set: %v", err)
	}

	// Generate JWT secret
	jwtSecret := []byte("test-jwt-secret-for-integration-testing-only")

	// Create API server
	params := apiserver.Params{
		FactorySet:     factorySet,
		EntityService:  services.NewEntityService(factorySet, "file://uploads?memfs=1&create_dir=1"),
		UploadLocation: "file://uploads?memfs=1&create_dir=1",
		DebugInfo:      debug.NewInfo(dsn, "file://uploads?memfs=1&create_dir=1"),
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

	return server, cleanup
}

// attemptLogin attempts to login with given credentials and returns success status
func attemptLogin(t *testing.T, serverURL, email, password string) bool {
	loginReq := map[string]string{
		"email":    email,
		"password": password,
	}
	reqBody, _ := json.Marshal(loginReq)

	resp, err := http.Post(serverURL+"/api/v1/auth/login", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		t.Logf("Login request failed: %v", err)
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// getAuthToken performs login and returns the JWT token
func getAuthToken(serverURL, email, password string) (string, error) {
	loginReq := map[string]string{
		"email":    email,
		"password": password,
	}
	reqBody, _ := json.Marshal(loginReq)

	resp, err := http.Post(serverURL+"/api/v1/auth/login", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login failed with status: %d", resp.StatusCode)
	}

	var loginResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", fmt.Errorf("failed to decode login response: %w", err)
	}

	return loginResp.Token, nil
}

// getSystemInfo accesses the system info API with the given token
func getSystemInfo(serverURL, token string) (map[string]any, error) {
	req, err := http.NewRequest("GET", serverURL+"/api/v1/system", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("system info request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("system info request failed with status: %d", resp.StatusCode)
	}

	var systemInfo map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&systemInfo); err != nil {
		return nil, fmt.Errorf("failed to decode system info response: %w", err)
	}

	return systemInfo, nil
}
