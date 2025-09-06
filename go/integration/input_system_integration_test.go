package integration_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	tenantcreate "github.com/denisvmedia/inventario/cmd/inventario/tenants/create"
	usercreate "github.com/denisvmedia/inventario/cmd/inventario/users/create"
)

// TestInputSystemIntegration tests the new interactive input system with both
// user and tenant create commands in interactive and non-interactive modes
func TestInputSystemIntegration(t *testing.T) {
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

	// Simulate user input for interactive mode
	// Input: tenant name, accept generated slug, optional domain
	simulatedInput := "Test Organization Interactive\n\nexample.com\n"

	dbConfig := &shared.DatabaseConfig{DBDSN: dsn}
	cmd := tenantcreate.New(dbConfig)

	// Capture output
	var output bytes.Buffer
	cmd.Cmd().SetOut(&output)
	cmd.Cmd().SetErr(&output)

	// Set up input simulation
	cmd.Cmd().SetIn(strings.NewReader(simulatedInput))

	// Set arguments for interactive mode
	cmd.Cmd().SetArgs([]string{
		"--db-dsn=" + dsn,
		"--interactive",
	})

	// Execute command
	err := cmd.Cmd().Execute()
	c.Assert(err, qt.IsNil, qt.Commentf("Tenant creation should succeed. Output: %s", output.String()))

	// Verify output contains expected prompts and success message
	outputStr := output.String()
	c.Assert(outputStr, qt.Contains, "Tenant name:")
	c.Assert(outputStr, qt.Contains, "Tenant slug")
	c.Assert(outputStr, qt.Contains, "✅ Tenant created successfully!")

	t.Log("✅ Interactive tenant creation completed successfully")
}

// testTenantCreateNonInteractive tests tenant creation in non-interactive mode
func testTenantCreateNonInteractive(t *testing.T, dsn string) {
	c := qt.New(t)

	t.Log("🏢 Testing tenant creation in non-interactive mode...")

	dbConfig := &shared.DatabaseConfig{DBDSN: dsn}
	cmd := tenantcreate.New(dbConfig)

	// Capture output
	var output bytes.Buffer
	cmd.Cmd().SetOut(&output)
	cmd.Cmd().SetErr(&output)

	// Set arguments for non-interactive mode
	cmd.Cmd().SetArgs([]string{
		"--db-dsn=" + dsn,
		"--name=Test Organization Non-Interactive",
		"--slug=test-org-non-interactive",
		"--domain=noninteractive.example.com",
		"--no-interactive",
	})

	// Execute command
	err := cmd.Cmd().Execute()
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

	// Simulate user input for interactive mode
	// Input: email, full name, password, confirm password, tenant slug
	simulatedInput := fmt.Sprintf("testuser@example.com\nTest User Interactive\nTestPassword123\nTestPassword123\n%s\n", tenantSlug)

	dbConfig := &shared.DatabaseConfig{DBDSN: dsn}
	cmd := usercreate.New(dbConfig)

	// Capture output
	var output bytes.Buffer
	cmd.Cmd().SetOut(&output)
	cmd.Cmd().SetErr(&output)

	// Set up input simulation
	cmd.Cmd().SetIn(strings.NewReader(simulatedInput))

	// Set arguments for interactive mode
	cmd.Cmd().SetArgs([]string{
		"--db-dsn=" + dsn,
		"--interactive",
	})

	// Execute command
	err := cmd.Cmd().Execute()
	c.Assert(err, qt.IsNil, qt.Commentf("User creation should succeed. Output: %s", output.String()))

	// Verify output contains expected prompts and success message
	outputStr := output.String()
	c.Assert(outputStr, qt.Contains, "Email:")
	c.Assert(outputStr, qt.Contains, "Full name:")
	c.Assert(outputStr, qt.Contains, "Password:")
	c.Assert(outputStr, qt.Contains, "✅ User created successfully!")

	t.Log("✅ Interactive user creation completed successfully")
}

// testUserCreateNonInteractive tests user creation in non-interactive mode
func testUserCreateNonInteractive(t *testing.T, dsn string) {
	c := qt.New(t)

	t.Log("👤 Testing user creation in non-interactive mode...")

	// First ensure we have a tenant to associate the user with
	tenantSlug := createTestTenant(t, dsn, "user-test-tenant-ni")

	dbConfig := &shared.DatabaseConfig{DBDSN: dsn}
	cmd := usercreate.New(dbConfig)

	// Capture output
	var output bytes.Buffer
	cmd.Cmd().SetOut(&output)
	cmd.Cmd().SetErr(&output)

	// Set arguments for non-interactive mode
	cmd.Cmd().SetArgs([]string{
		"--db-dsn=" + dsn,
		"--email=testuser-ni@example.com",
		"--name=Test User Non-Interactive",
		"--password=TestPassword123",
		"--tenant=" + tenantSlug,
		"--role=user",
		"--no-interactive",
	})

	// Execute command
	err := cmd.Cmd().Execute()
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

	// Test invalid email followed by valid email
	simulatedInput := "invalid-email\ntestvalidation@example.com\nTest Validation User\nTestPassword123\nTestPassword123\nuser-test-tenant\n"

	dbConfig := &shared.DatabaseConfig{DBDSN: dsn}
	cmd := usercreate.New(dbConfig)

	// Capture output
	var output bytes.Buffer
	cmd.Cmd().SetOut(&output)
	cmd.Cmd().SetErr(&output)

	// Set up input simulation
	cmd.Cmd().SetIn(strings.NewReader(simulatedInput))

	// Set arguments for interactive mode
	cmd.Cmd().SetArgs([]string{
		"--db-dsn=" + dsn,
		"--interactive",
	})

	// Execute command
	err := cmd.Cmd().Execute()
	c.Assert(err, qt.IsNil, qt.Commentf("User creation should succeed after validation error. Output: %s", output.String()))

	// Verify output contains validation error and re-prompting
	outputStr := output.String()
	c.Assert(outputStr, qt.Contains, "Error:")
	c.Assert(outputStr, qt.Contains, "✅ User created successfully!")

	t.Log("✅ Validation error handling completed successfully")
}

// testDefaultValueHandling tests that default values work correctly in interactive mode
func testDefaultValueHandling(t *testing.T, dsn string) {
	c := qt.New(t)

	t.Log("🔧 Testing default value handling...")

	// Test tenant creation with default slug generation
	// Input: tenant name, press enter to accept default slug, skip domain
	simulatedInput := "Test Default Values\n\n\n"

	dbConfig := &shared.DatabaseConfig{DBDSN: dsn}
	cmd := tenantcreate.New(dbConfig)

	var output bytes.Buffer
	cmd.Cmd().SetOut(&output)
	cmd.Cmd().SetErr(&output)

	cmd.Cmd().SetIn(strings.NewReader(simulatedInput))

	cmd.Cmd().SetArgs([]string{
		"--db-dsn=" + dsn,
		"--interactive",
	})

	err := cmd.Cmd().Execute()
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

	// Test weak password followed by strong password
	// Input: email, name, weak password, strong password, confirm strong password, tenant
	simulatedInput := fmt.Sprintf("passwordtest@example.com\nPassword Test User\nweak\nStrongPassword123\nStrongPassword123\n%s\n", tenantSlug)

	dbConfig := &shared.DatabaseConfig{DBDSN: dsn}
	cmd := usercreate.New(dbConfig)

	var output bytes.Buffer
	cmd.Cmd().SetOut(&output)
	cmd.Cmd().SetErr(&output)

	cmd.Cmd().SetIn(strings.NewReader(simulatedInput))

	cmd.Cmd().SetArgs([]string{
		"--db-dsn=" + dsn,
		"--interactive",
	})

	err := cmd.Cmd().Execute()
	c.Assert(err, qt.IsNil, qt.Commentf("User creation should succeed after password validation. Output: %s", output.String()))

	outputStr := output.String()
	c.Assert(outputStr, qt.Contains, "password must be at least 8 characters long")
	c.Assert(outputStr, qt.Contains, "✅ User created successfully!")

	t.Log("✅ Password validation completed successfully")
}

// testSlugGeneration tests automatic slug generation and validation
func testSlugGeneration(t *testing.T, dsn string) {
	c := qt.New(t)

	t.Log("🏷️ Testing slug generation and validation...")

	// Test with special characters that should be converted to slug format
	simulatedInput := "Test Organization With Special Characters!@#\n\n\n"

	dbConfig := &shared.DatabaseConfig{DBDSN: dsn}
	cmd := tenantcreate.New(dbConfig)

	var output bytes.Buffer
	cmd.Cmd().SetOut(&output)
	cmd.Cmd().SetErr(&output)

	cmd.Cmd().SetIn(strings.NewReader(simulatedInput))

	cmd.Cmd().SetArgs([]string{
		"--db-dsn=" + dsn,
		"--interactive",
	})

	err := cmd.Cmd().Execute()
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
	dbConfig := &shared.DatabaseConfig{DBDSN: dsn}
	cmd := tenantcreate.New(dbConfig)

	var output bytes.Buffer
	cmd.Cmd().SetOut(&output)
	cmd.Cmd().SetErr(&output)

	// Generate unique tenant name with timestamp
	uniqueName := fmt.Sprintf("%s-%d", tenantName, time.Now().Unix())
	slug := strings.ReplaceAll(strings.ToLower(uniqueName), " ", "-")

	cmd.Cmd().SetArgs([]string{
		"--db-dsn=" + dsn,
		"--name=" + uniqueName,
		"--slug=" + slug,
		"--no-interactive",
	})

	err := cmd.Cmd().Execute()
	if err != nil {
		t.Fatalf("Failed to create test tenant: %v\nOutput: %s", err, output.String())
	}

	return slug
}
