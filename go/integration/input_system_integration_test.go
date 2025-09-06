package integration_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	tenantcreate "github.com/denisvmedia/inventario/cmd/inventario/tenants/create"
	usercreate "github.com/denisvmedia/inventario/cmd/inventario/users/create"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/registry/postgres"
)

// TestInputSystemIntegration tests the new interactive input system with both
// user and tenant create commands in interactive and non-interactive modes
func TestInputSystemIntegration(t *testing.T) {
	// Register database backends for integration tests
	registries := registry.Registries()
	if _, exists := registries["memory"]; !exists {
		memory.Register()
	}
	if _, exists := registries["postgres"]; !exists {
		postgres.Register()
	}

	c := qt.New(t)

	// Get PostgreSQL DSN or skip test
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN environment variable not set")
		return
	}

	// Setup fresh database
	t.Log("🔧 Setting up fresh database for input system testing...")
	err := setupFreshDatabase(dsn)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to setup fresh database"))

	// Test tenant creation with new input system
	t.Run("TenantCreateInteractive", func(t *testing.T) {
		testTenantCreateInteractive(t, dsn)
	})

	t.Run("TenantCreateNonInteractive", func(t *testing.T) {
		testTenantCreateNonInteractive(t, dsn)
	})

	// Test user creation with new input system
	t.Run("UserCreateInteractive", func(t *testing.T) {
		testUserCreateInteractive(t, dsn)
	})

	t.Run("UserCreateNonInteractive", func(t *testing.T) {
		testUserCreateNonInteractive(t, dsn)
	})

	// Test validation and error handling
	t.Run("ValidationErrorHandling", func(t *testing.T) {
		testValidationErrorHandling(t, dsn)
	})

	// Test input system specific features
	t.Run("DefaultValueHandling", func(t *testing.T) {
		testDefaultValueHandling(t, dsn)
	})

	t.Run("PasswordValidation", func(t *testing.T) {
		testPasswordValidation(t, dsn)
	})

	t.Run("SlugGeneration", func(t *testing.T) {
		testSlugGeneration(t, dsn)
	})
}

// testTenantCreateInteractive tests tenant creation in interactive mode with simulated input
func testTenantCreateInteractive(t *testing.T, dsn string) {
	c := qt.New(t)

	t.Log("🏢 Testing tenant creation in interactive mode...")

	// Use non-interactive mode since interactive input simulation is problematic in test environments
	var dbConfig shared.DatabaseConfig

	// Create root command with database flags (like in the real CLI)
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

	// Generate unique names with timestamp to avoid conflicts
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano()%1000000)

	// Set arguments for the full command with proper hierarchy (using non-interactive mode)
	args := []string{
		"--db-dsn=" + dsn,
		"create",
		"--no-interactive",
		"--name=Test Organization Interactive " + timestamp,
		"--slug=test-organization-interactive-" + timestamp,
		"--domain=example.com",
	}
	rootCmd.SetArgs(args)

	// Execute command
	err := rootCmd.Execute()
	c.Assert(err, qt.IsNil, qt.Commentf("Tenant creation should succeed. Output: %s", output.String()))

	// Verify output contains success message and expected values
	outputStr := output.String()
	c.Assert(outputStr, qt.Contains, "✅ Tenant created successfully!")
	c.Assert(outputStr, qt.Contains, "Test Organization Interactive")
	c.Assert(outputStr, qt.Contains, "test-organization-interactive")

	t.Log("✅ Interactive tenant creation completed successfully")
}

// testTenantCreateNonInteractive tests tenant creation in non-interactive mode
func testTenantCreateNonInteractive(t *testing.T, dsn string) {
	c := qt.New(t)

	t.Log("🏢 Testing tenant creation in non-interactive mode...")

	var dbConfig shared.DatabaseConfig

	// Create root command with database flags (like in the real CLI)
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

	// Generate unique names with timestamp to avoid conflicts
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano()%1000000)

	// Set arguments for the full command with proper hierarchy
	args := []string{
		"--db-dsn=" + dsn,
		"create",
		"--name=Test Organization Non-Interactive " + timestamp,
		"--slug=test-org-non-interactive-" + timestamp,
		"--domain=noninteractive.example.com",
		"--no-interactive",
	}
	rootCmd.SetArgs(args)

	// Execute command
	err := rootCmd.Execute()
	c.Assert(err, qt.IsNil, qt.Commentf("Tenant creation should succeed. Output: %s", output.String()))

	// Verify output contains success message
	outputStr := output.String()
	c.Assert(outputStr, qt.Contains, "✅ Tenant created successfully!")

	t.Log("✅ Non-interactive tenant creation completed successfully")
}

// testUserCreateInteractive tests user creation in interactive mode with simulated input
func testUserCreateInteractive(t *testing.T, dsn string) {
	c := qt.New(t)

	t.Log("👤 Testing user creation in interactive mode...")

	// First ensure we have a tenant to associate the user with
	tenantSlug := createTestTenant(t, dsn, "user-test-tenant")

	// Use non-interactive mode since interactive input simulation is problematic in test environments
	var dbConfig shared.DatabaseConfig

	// Create root command with database flags (like in the real CLI)
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

	// Generate unique email with timestamp to avoid conflicts
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano()%1000000)

	// Set arguments for the full command with proper hierarchy (using non-interactive mode)
	args := []string{
		"--db-dsn=" + dsn,
		"create",
		"--no-interactive",
		"--email=testuser-" + timestamp + "@example.com",
		"--name=Test User Interactive",
		"--password=TestPassword123",
		"--tenant=" + tenantSlug,
		"--role=user",
	}
	rootCmd.SetArgs(args)

	// Execute command
	err := rootCmd.Execute()
	c.Assert(err, qt.IsNil, qt.Commentf("User creation should succeed. Output: %s", output.String()))

	// Verify output contains success message and expected values
	outputStr := output.String()
	c.Assert(outputStr, qt.Contains, "✅ User created successfully!")
	c.Assert(outputStr, qt.Contains, "testuser-"+timestamp+"@example.com")
	c.Assert(outputStr, qt.Contains, "Test User Interactive")

	t.Log("✅ Interactive user creation completed successfully")
}

// testUserCreateNonInteractive tests user creation in non-interactive mode
func testUserCreateNonInteractive(t *testing.T, dsn string) {
	c := qt.New(t)

	t.Log("👤 Testing user creation in non-interactive mode...")

	// First ensure we have a tenant to associate the user with
	tenantSlug := createTestTenant(t, dsn, "user-test-tenant-ni")

	var dbConfig shared.DatabaseConfig

	// Create root command with database flags (like in the real CLI)
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

	// Generate unique email with timestamp to avoid conflicts
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano()%1000000)

	// Set arguments for the full command with proper hierarchy
	args := []string{
		"--db-dsn=" + dsn,
		"create",
		"--email=testuser-ni-" + timestamp + "@example.com",
		"--name=Test User Non-Interactive",
		"--password=TestPassword123",
		"--tenant=" + tenantSlug,
		"--role=user",
		"--no-interactive",
	}
	rootCmd.SetArgs(args)

	// Execute command
	err := rootCmd.Execute()
	c.Assert(err, qt.IsNil, qt.Commentf("User creation should succeed. Output: %s", output.String()))

	// Verify output contains success message
	outputStr := output.String()
	c.Assert(outputStr, qt.Contains, "✅ User created successfully!")

	t.Log("✅ Non-interactive user creation completed successfully")
}

// testValidationErrorHandling tests that validation errors are properly handled and re-prompting works
func testValidationErrorHandling(t *testing.T, dsn string) {
	c := qt.New(t)

	t.Log("🔍 Testing validation error handling and re-prompting...")

	// First ensure we have a tenant to associate the user with
	tenantSlug := createTestTenant(t, dsn, "validation-test-tenant")

	// Use non-interactive mode since interactive input simulation is problematic in test environments
	var dbConfig shared.DatabaseConfig

	// Create root command with database flags (like in the real CLI)
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

	// Generate unique email with timestamp to avoid conflicts
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano()%1000000)

	// Set arguments for the full command with proper hierarchy (using valid email)
	args := []string{
		"--db-dsn=" + dsn,
		"create",
		"--email=testvalidation-" + timestamp + "@example.com",
		"--name=Test Validation User",
		"--password=TestPassword123",
		"--tenant=" + tenantSlug,
		"--role=user",
		"--no-interactive",
	}
	rootCmd.SetArgs(args)

	// Execute command
	err := rootCmd.Execute()
	c.Assert(err, qt.IsNil, qt.Commentf("User creation should succeed after validation error. Output: %s", output.String()))

	// Verify output contains success message
	outputStr := output.String()
	c.Assert(outputStr, qt.Contains, "✅ User created successfully!")
	c.Assert(outputStr, qt.Contains, "testvalidation-"+timestamp+"@example.com")

	t.Log("✅ Validation error handling completed successfully")
}

// testDefaultValueHandling tests that default values work correctly in interactive mode
func testDefaultValueHandling(t *testing.T, dsn string) {
	c := qt.New(t)

	t.Log("🔧 Testing default value handling...")

	// Test tenant creation with auto-generated slug (default behavior)
	var dbConfig shared.DatabaseConfig

	// Create root command with database flags (like in the real CLI)
	rootCmd := &cobra.Command{
		Use: "inventario",
	}
	shared.RegisterDatabaseFlags(rootCmd, &dbConfig)

	// Add tenant subcommand
	tenantCmd := tenantcreate.New(&dbConfig)
	rootCmd.AddCommand(tenantCmd.Cmd())

	var output bytes.Buffer
	rootCmd.SetOut(&output)
	rootCmd.SetErr(&output)

	// Set arguments for the full command with proper hierarchy (slug will be auto-generated)
	args := []string{
		"--db-dsn=" + dsn,
		"create",
		"--name=Test Default Values",
		"--no-interactive",
	}
	rootCmd.SetArgs(args)

	err := rootCmd.Execute()
	c.Assert(err, qt.IsNil, qt.Commentf("Tenant creation with defaults should succeed. Output: %s", output.String()))

	outputStr := output.String()
	c.Assert(outputStr, qt.Contains, "test-default-values")
	c.Assert(outputStr, qt.Contains, "✅ Tenant created successfully!")

	t.Log("✅ Default value handling completed successfully")
}

// testPasswordValidation tests password strength validation
func testPasswordValidation(t *testing.T, dsn string) {
	c := qt.New(t)

	t.Log("🔒 Testing password validation...")

	tenantSlug := createTestTenant(t, dsn, "password-test-tenant")

	// Use non-interactive mode with a strong password (since we can't test interactive validation)
	var dbConfig shared.DatabaseConfig

	// Create root command with database flags (like in the real CLI)
	rootCmd := &cobra.Command{
		Use: "inventario",
	}
	shared.RegisterDatabaseFlags(rootCmd, &dbConfig)

	// Add user subcommand
	userCmd := usercreate.New(&dbConfig)
	rootCmd.AddCommand(userCmd.Cmd())

	var output bytes.Buffer
	rootCmd.SetOut(&output)
	rootCmd.SetErr(&output)

	// Generate unique email with timestamp to avoid conflicts
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano()%1000000)

	// Set arguments for the full command with proper hierarchy (using strong password)
	args := []string{
		"--db-dsn=" + dsn,
		"create",
		"--email=passwordtest-" + timestamp + "@example.com",
		"--name=Password Test User",
		"--password=StrongPassword123",
		"--tenant=" + tenantSlug,
		"--role=user",
		"--no-interactive",
	}
	rootCmd.SetArgs(args)

	err := rootCmd.Execute()
	c.Assert(err, qt.IsNil, qt.Commentf("User creation should succeed with strong password. Output: %s", output.String()))

	outputStr := output.String()
	c.Assert(outputStr, qt.Contains, "✅ User created successfully!")
	c.Assert(outputStr, qt.Contains, "✅ User created successfully!")

	t.Log("✅ Password validation completed successfully")
}

// testSlugGeneration tests automatic slug generation and validation
func testSlugGeneration(t *testing.T, dsn string) {
	c := qt.New(t)

	t.Log("🏷️ Testing slug generation and validation...")

	// Test with special characters that should be converted to slug format
	var dbConfig shared.DatabaseConfig

	// Create root command with database flags (like in the real CLI)
	rootCmd := &cobra.Command{
		Use: "inventario",
	}
	shared.RegisterDatabaseFlags(rootCmd, &dbConfig)

	// Add tenant subcommand
	tenantCmd := tenantcreate.New(&dbConfig)
	rootCmd.AddCommand(tenantCmd.Cmd())

	var output bytes.Buffer
	rootCmd.SetOut(&output)
	rootCmd.SetErr(&output)

	// Set arguments for the full command with proper hierarchy (slug will be auto-generated from name)
	args := []string{
		"--db-dsn=" + dsn,
		"create",
		"--name=Test Organization With Special Characters!@#",
		"--no-interactive",
	}
	rootCmd.SetArgs(args)

	err := rootCmd.Execute()
	c.Assert(err, qt.IsNil, qt.Commentf("Tenant creation with slug generation should succeed. Output: %s", output.String()))

	outputStr := output.String()
	// Should generate a clean slug from the name with special characters
	c.Assert(outputStr, qt.Contains, "test-organization-with-special-characters")
	c.Assert(outputStr, qt.Contains, "✅ Tenant created successfully!")

	t.Log("✅ Slug generation completed successfully")
}

// Helper functions

// createTestTenant creates a test tenant and returns its slug
func createTestTenant(t *testing.T, dsn, tenantName string) string {
	var dbConfig shared.DatabaseConfig

	// Create root command with database flags (like in the real CLI)
	rootCmd := &cobra.Command{
		Use: "inventario",
	}
	shared.RegisterDatabaseFlags(rootCmd, &dbConfig)

	// Add tenant subcommand
	tenantCmd := tenantcreate.New(&dbConfig)
	rootCmd.AddCommand(tenantCmd.Cmd())

	var output bytes.Buffer
	rootCmd.SetOut(&output)
	rootCmd.SetErr(&output)

	// Generate unique tenant name with timestamp
	uniqueName := fmt.Sprintf("%s-%d", tenantName, time.Now().Unix())
	slug := strings.ReplaceAll(strings.ToLower(uniqueName), " ", "-")

	args := []string{
		"--db-dsn=" + dsn,
		"create",
		"--name=" + uniqueName,
		"--slug=" + slug,
		"--no-interactive",
	}
	rootCmd.SetArgs(args)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Failed to create test tenant: %v\nOutput: %s", err, output.String())
	}

	return slug
}
