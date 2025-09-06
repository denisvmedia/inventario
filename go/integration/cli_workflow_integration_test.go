//go:build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/frankban/quicktest"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/cmd/inventario/db/bootstrap/apply"
	"github.com/denisvmedia/inventario/cmd/inventario/db/migrate/up"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	tenantcreate "github.com/denisvmedia/inventario/cmd/inventario/tenants/create"
	usercreate "github.com/denisvmedia/inventario/cmd/inventario/users/create"
	"github.com/denisvmedia/inventario/debug"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
	"github.com/denisvmedia/inventario/services"
)

// TestCLIWorkflowIntegration tests the complete workflow from fresh database setup
// through CLI operations to API access, simulating a CI pipeline scenario
func TestCLIWorkflowIntegration(t *testing.T) {
	c := quicktest.New(t)

	// Get PostgreSQL DSN or skip test
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN environment variable not set")
		return
	}

	// Step 1: Setup fresh database with bootstrap and migrations
	t.Log("🔧 Setting up fresh database with bootstrap and migrations...")
	err := setupFreshDatabase(dsn)
	c.Assert(err, quicktest.IsNil, quicktest.Commentf("Failed to setup fresh database"))

	// Step 2: Attempt login with non-existent user (should fail)
	t.Log("🔐 Testing login with non-existent user (should fail)...")
	server, cleanup := setupAPIServer(t, dsn)
	defer cleanup()

	loginSuccess := attemptLogin(t, server.URL, "nonexistent@example.com", "password123")
	c.Assert(loginSuccess, quicktest.IsFalse, quicktest.Commentf("Login should fail for non-existent user"))

	// Step 3: Create tenant via CLI
	t.Log("🏢 Creating tenant via CLI...")
	tenantSlug := "test-company"
	err = createTenantViaCLI(dsn, "Test Company", tenantSlug, "test-company.com")
	c.Assert(err, quicktest.IsNil, quicktest.Commentf("Failed to create tenant via CLI"))

	// Step 4: Create user via CLI
	t.Log("👤 Creating user via CLI...")
	userEmail := "admin@test-company.com"
	userPassword := "SecurePassword123!"
	err = createUserViaCLI(dsn, userEmail, userPassword, "Admin User", tenantSlug, "admin")
	c.Assert(err, quicktest.IsNil, quicktest.Commentf("Failed to create user via CLI"))

	// Step 5: Attempt login with created user (should succeed)
	t.Log("🔐 Testing login with created user (should succeed)...")
	loginSuccess = attemptLogin(t, server.URL, userEmail, userPassword)
	c.Assert(loginSuccess, quicktest.IsTrue, quicktest.Commentf("Login should succeed for created user"))

	// Step 6: Get authentication token
	t.Log("🎫 Getting authentication token...")
	token, err := getAuthToken(server.URL, userEmail, userPassword)
	c.Assert(err, quicktest.IsNil, quicktest.Commentf("Failed to get authentication token"))
	c.Assert(token, quicktest.Not(quicktest.Equals), "", quicktest.Commentf("Token should not be empty"))

	// Step 7: Access system info API with valid token (should succeed)
	t.Log("📊 Testing system info API access with valid token...")
	systemInfo, err := getSystemInfo(server.URL, token)
	c.Assert(err, quicktest.IsNil, quicktest.Commentf("Failed to access system info API"))
	c.Assert(systemInfo, quicktest.IsNotNil, quicktest.Commentf("System info should not be nil"))
	c.Assert(systemInfo["database_backend"], quicktest.Equals, "postgres", quicktest.Commentf("Database backend should be postgres"))

	t.Log("✅ CLI workflow integration test completed successfully!")
}

// setupFreshDatabase runs bootstrap and migration commands to set up a fresh database
func setupFreshDatabase(dsn string) error {
	// Step 1: Run bootstrap migrations
	dbConfig := &shared.DatabaseConfig{DBDSN: dsn}
	bootstrapCmd := apply.New(dbConfig)

	// Register the database flags for the bootstrap command
	shared.RegisterLocalDatabaseFlags(bootstrapCmd.Cmd(), dbConfig)

	// Capture output
	var bootstrapOutput bytes.Buffer
	bootstrapCmd.Cmd().SetOut(&bootstrapOutput)
	bootstrapCmd.Cmd().SetErr(&bootstrapOutput)

	// Set bootstrap arguments
	bootstrapCmd.Cmd().SetArgs([]string{
		"--db-dsn=" + dsn,
		"--username=inventario",
		"--username-for-migrations=inventario",
	})

	if err := bootstrapCmd.Cmd().Execute(); err != nil {
		return fmt.Errorf("bootstrap failed: %w\nOutput: %s", err, bootstrapOutput.String())
	}

	// Step 2: Run schema migrations
	migrateCmd := up.New(dbConfig)

	// Register the database flags for the migration command
	shared.RegisterLocalDatabaseFlags(migrateCmd.Cmd(), dbConfig)

	// Capture output
	var migrateOutput bytes.Buffer
	migrateCmd.Cmd().SetOut(&migrateOutput)
	migrateCmd.Cmd().SetErr(&migrateOutput)

	// Set migration arguments
	migrateCmd.Cmd().SetArgs([]string{
		"--db-dsn=" + dsn,
	})

	if err := migrateCmd.Cmd().Execute(); err != nil {
		return fmt.Errorf("migration failed: %w\nOutput: %s", err, migrateOutput.String())
	}

	return nil
}

// createTenantViaCLI creates a tenant using the CLI command
func createTenantViaCLI(dsn, name, slug, domain string) error {
	var dbConfig shared.DatabaseConfig

	// Create root command with database flags
	rootCmd := &cobra.Command{
		Use: "inventario",
	}
	shared.RegisterDatabaseFlags(rootCmd, &dbConfig)

	// Add tenant subcommand
	tenantCmd := tenantcreate.New(&dbConfig)
	rootCmd.AddCommand(tenantCmd.Cmd())

	// Capture output
	var output bytes.Buffer
	rootCmd.SetOut(&output)
	rootCmd.SetErr(&output)

	// Set arguments for the full command
	rootCmd.SetArgs([]string{
		"--db-dsn=" + dsn,
		"create",
		"--name=" + name,
		"--slug=" + slug,
		"--domain=" + domain,
		"--no-interactive",
	})

	if err := rootCmd.Execute(); err != nil {
		return fmt.Errorf("tenant creation failed: %w\nOutput: %s", err, output.String())
	}

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
	rootCmd.SetArgs([]string{
		"--db-dsn=" + dsn,
		"create",
		"--email=" + email,
		"--password=" + password,
		"--name=" + name,
		"--tenant=" + tenant,
		"--role=" + role,
		"--no-interactive",
	})

	if err := rootCmd.Execute(); err != nil {
		return fmt.Errorf("user creation failed: %w\nOutput: %s", err, output.String())
	}

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
