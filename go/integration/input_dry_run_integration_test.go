//go:build integration

package integration_test

import (
	"bytes"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	tenantcreate "github.com/denisvmedia/inventario/cmd/inventario/tenants/create"
	usercreate "github.com/denisvmedia/inventario/cmd/inventario/users/create"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/registry/postgres"
)

func init() {
	// Register database backends for integration tests
	memory.Register()
	postgres.Register()
}

// TestInputSystemDryRunIntegration tests the new interactive input system
// using dry-run mode to avoid requiring a database connection
func TestInputSystemDryRunIntegration(t *testing.T) {
	c := qt.New(t)

	t.Log("🧪 Testing input system integration with dry-run mode...")

	// Test tenant creation with new input system
	t.Run("TenantCreateInteractiveDryRun", func(t *testing.T) {
		testTenantCreateInteractiveDryRun(t)
	})

	t.Run("TenantCreateNonInteractiveDryRun", func(t *testing.T) {
		testTenantCreateNonInteractiveDryRun(t)
	})

	// Test user creation with new input system
	t.Run("UserCreateInteractiveDryRun", func(t *testing.T) {
		testUserCreateInteractiveDryRun(t)
	})

	t.Run("UserCreateNonInteractiveDryRun", func(t *testing.T) {
		testUserCreateNonInteractiveDryRun(t)
	})

	// Test validation scenarios
	t.Run("ValidationErrorHandlingDryRun", func(t *testing.T) {
		testValidationErrorHandlingDryRun(t)
	})

	c.Log("✅ All input system dry-run integration tests completed successfully")
}

// testTenantCreateInteractiveDryRun tests tenant creation in interactive mode with dry-run
func testTenantCreateInteractiveDryRun(t *testing.T) {
	c := qt.New(t)

	t.Log("🏢 Testing tenant creation in interactive mode (dry-run)...")

	// Simulate user input for interactive mode
	// Input: tenant name, accept generated slug, optional domain
	simulatedInput := "Test Organization Interactive\n\nexample.com\n"

	dbConfig := &shared.DatabaseConfig{DBDSN: "postgres://user:pass@localhost/db"}
	cmd := tenantcreate.New(dbConfig)

	// Capture output
	var output bytes.Buffer
	cmd.Cmd().SetOut(&output)
	cmd.Cmd().SetErr(&output)

	// Set up input simulation
	cmd.Cmd().SetIn(strings.NewReader(simulatedInput))

	// Set arguments for interactive mode with dry-run
	cmd.Cmd().SetArgs([]string{
		"--dry-run",
		"--interactive",
	})

	// Execute command
	err := cmd.Cmd().Execute()
	c.Assert(err, qt.IsNil, qt.Commentf("Tenant creation should succeed in dry-run. Output: %s", output.String()))

	// Verify output contains expected prompts and dry-run message
	outputStr := output.String()
	c.Assert(outputStr, qt.Contains, "Tenant name:")
	c.Assert(outputStr, qt.Contains, "Tenant slug")
	c.Assert(outputStr, qt.Contains, "🔍 DRY RUN MODE")
	c.Assert(outputStr, qt.Contains, "Name:     Test Organization Interactive")
	c.Assert(outputStr, qt.Contains, "Slug:     test-organization-interactive")
	c.Assert(outputStr, qt.Contains, "Domain:   example.com")

	t.Log("✅ Interactive tenant creation (dry-run) completed successfully")
}

// testTenantCreateNonInteractiveDryRun tests tenant creation in non-interactive mode with dry-run
func testTenantCreateNonInteractiveDryRun(t *testing.T) {
	c := qt.New(t)

	t.Log("🏢 Testing tenant creation in non-interactive mode (dry-run)...")

	dbConfig := &shared.DatabaseConfig{DBDSN: "postgres://user:pass@localhost/db"}
	cmd := tenantcreate.New(dbConfig)

	// Capture output
	var output bytes.Buffer
	cmd.Cmd().SetOut(&output)
	cmd.Cmd().SetErr(&output)

	// Set arguments for non-interactive mode with dry-run
	cmd.Cmd().SetArgs([]string{
		"--dry-run",
		"--name=Test Organization Non-Interactive",
		"--slug=test-org-non-interactive",
		"--domain=noninteractive.example.com",
		"--no-interactive",
	})

	// Execute command
	err := cmd.Cmd().Execute()
	c.Assert(err, qt.IsNil, qt.Commentf("Tenant creation should succeed in dry-run. Output: %s", output.String()))

	// Verify output contains dry-run message and expected values
	outputStr := output.String()
	c.Assert(outputStr, qt.Contains, "🔍 DRY RUN MODE")
	c.Assert(outputStr, qt.Contains, "Name:     Test Organization Non-Interactive")
	c.Assert(outputStr, qt.Contains, "Slug:     test-org-non-interactive")
	c.Assert(outputStr, qt.Contains, "Domain:   noninteractive.example.com")

	t.Log("✅ Non-interactive tenant creation (dry-run) completed successfully")
}

// testUserCreateInteractiveDryRun tests user creation in interactive mode with dry-run
func testUserCreateInteractiveDryRun(t *testing.T) {
	c := qt.New(t)

	t.Log("👤 Testing user creation in interactive mode (dry-run)...")

	// Simulate user input for interactive mode
	// Input: email, full name, password, confirm password, tenant slug
	simulatedInput := "testuser@example.com\nTest User Interactive\nTestPassword123\nTestPassword123\ntest-tenant\n"

	dbConfig := &shared.DatabaseConfig{DBDSN: "postgres://user:pass@localhost/db"}
	cmd := usercreate.New(dbConfig)

	// Capture output
	var output bytes.Buffer
	cmd.Cmd().SetOut(&output)
	cmd.Cmd().SetErr(&output)

	// Set up input simulation
	cmd.Cmd().SetIn(strings.NewReader(simulatedInput))

	// Set arguments for interactive mode with dry-run
	cmd.Cmd().SetArgs([]string{
		"--dry-run",
		"--interactive",
	})

	// Execute command
	err := cmd.Cmd().Execute()
	c.Assert(err, qt.IsNil, qt.Commentf("User creation should succeed in dry-run. Output: %s", output.String()))

	// Verify output contains expected prompts and dry-run message
	outputStr := output.String()
	c.Assert(outputStr, qt.Contains, "Email:")
	c.Assert(outputStr, qt.Contains, "Full name:")
	c.Assert(outputStr, qt.Contains, "Password:")
	c.Assert(outputStr, qt.Contains, "🔍 DRY RUN MODE")
	c.Assert(outputStr, qt.Contains, "Email:    testuser@example.com")
	c.Assert(outputStr, qt.Contains, "Name:     Test User Interactive")
	c.Assert(outputStr, qt.Contains, "Tenant:   test-tenant")

	t.Log("✅ Interactive user creation (dry-run) completed successfully")
}

// testUserCreateNonInteractiveDryRun tests user creation in non-interactive mode with dry-run
func testUserCreateNonInteractiveDryRun(t *testing.T) {
	c := qt.New(t)

	t.Log("👤 Testing user creation in non-interactive mode (dry-run)...")

	dbConfig := &shared.DatabaseConfig{DBDSN: "postgres://user:pass@localhost/db"}
	cmd := usercreate.New(dbConfig)

	// Capture output
	var output bytes.Buffer
	cmd.Cmd().SetOut(&output)
	cmd.Cmd().SetErr(&output)

	// Set arguments for non-interactive mode with dry-run
	cmd.Cmd().SetArgs([]string{
		"--dry-run",
		"--email=testuser-ni@example.com",
		"--name=Test User Non-Interactive",
		"--password=TestPassword123",
		"--tenant=test-tenant-ni",
		"--role=user",
		"--no-interactive",
	})

	// Execute command
	err := cmd.Cmd().Execute()
	c.Assert(err, qt.IsNil, qt.Commentf("User creation should succeed in dry-run. Output: %s", output.String()))

	// Verify output contains dry-run message and expected values
	outputStr := output.String()
	c.Assert(outputStr, qt.Contains, "🔍 DRY RUN MODE")
	c.Assert(outputStr, qt.Contains, "Email:    testuser-ni@example.com")
	c.Assert(outputStr, qt.Contains, "Name:     Test User Non-Interactive")
	c.Assert(outputStr, qt.Contains, "Tenant:   test-tenant-ni")
	c.Assert(outputStr, qt.Contains, "Role:     user")

	t.Log("✅ Non-interactive user creation (dry-run) completed successfully")
}

// testValidationErrorHandlingDryRun tests validation error handling in dry-run mode
func testValidationErrorHandlingDryRun(t *testing.T) {
	c := qt.New(t)

	t.Log("🔍 Testing validation error handling (dry-run)...")

	// Test invalid email followed by valid email
	simulatedInput := "invalid-email\ntestvalidation@example.com\nTest Validation User\nTestPassword123\nTestPassword123\ntest-tenant\n"

	dbConfig := &shared.DatabaseConfig{DBDSN: "postgres://user:pass@localhost/db"}
	cmd := usercreate.New(dbConfig)

	// Capture output
	var output bytes.Buffer
	cmd.Cmd().SetOut(&output)
	cmd.Cmd().SetErr(&output)

	// Set up input simulation
	cmd.Cmd().SetIn(strings.NewReader(simulatedInput))

	// Set arguments for interactive mode with dry-run
	cmd.Cmd().SetArgs([]string{
		"--dry-run",
		"--interactive",
	})

	// Execute command
	err := cmd.Cmd().Execute()
	c.Assert(err, qt.IsNil, qt.Commentf("User creation should succeed after validation error in dry-run. Output: %s", output.String()))

	// Verify output contains validation error and re-prompting, then success
	outputStr := output.String()
	c.Assert(outputStr, qt.Contains, "Error:")
	c.Assert(outputStr, qt.Contains, "must be a valid email address")
	c.Assert(outputStr, qt.Contains, "🔍 DRY RUN MODE")
	c.Assert(outputStr, qt.Contains, "Email:    testvalidation@example.com")

	t.Log("✅ Validation error handling (dry-run) completed successfully")
}

// Additional test for password validation
func TestPasswordValidationDryRun(t *testing.T) {
	c := qt.New(t)

	t.Log("🔒 Testing password validation in dry-run mode...")

	// Test weak password followed by strong password
	simulatedInput := "passwordtest@example.com\nPassword Test User\nweak\nStrongPassword123\nStrongPassword123\ntest-tenant\n"

	dbConfig := &shared.DatabaseConfig{DBDSN: "postgres://user:pass@localhost/db"}
	cmd := usercreate.New(dbConfig)

	var output bytes.Buffer
	cmd.Cmd().SetOut(&output)
	cmd.Cmd().SetErr(&output)

	cmd.Cmd().SetIn(strings.NewReader(simulatedInput))

	cmd.Cmd().SetArgs([]string{
		"--dry-run",
		"--interactive",
	})

	err := cmd.Cmd().Execute()
	c.Assert(err, qt.IsNil, qt.Commentf("User creation should succeed after password validation in dry-run. Output: %s", output.String()))

	outputStr := output.String()
	c.Assert(outputStr, qt.Contains, "Error:")
	c.Assert(outputStr, qt.Contains, "password must be at least 8 characters long")
	c.Assert(outputStr, qt.Contains, "🔍 DRY RUN MODE")

	t.Log("✅ Password validation (dry-run) completed successfully")
}

// Test for slug generation and validation
func TestSlugGenerationDryRun(t *testing.T) {
	c := qt.New(t)

	t.Log("🏷️ Testing slug generation in dry-run mode...")

	// Test with special characters that should be converted to slug format
	simulatedInput := "Test Organization With Special Characters!@#\n\n\n"

	dbConfig := &shared.DatabaseConfig{DBDSN: "postgres://user:pass@localhost/db"}
	cmd := tenantcreate.New(dbConfig)

	var output bytes.Buffer
	cmd.Cmd().SetOut(&output)
	cmd.Cmd().SetErr(&output)

	cmd.Cmd().SetIn(strings.NewReader(simulatedInput))

	cmd.Cmd().SetArgs([]string{
		"--dry-run",
		"--interactive",
	})

	err := cmd.Cmd().Execute()
	c.Assert(err, qt.IsNil, qt.Commentf("Tenant creation with slug generation should succeed in dry-run. Output: %s", output.String()))

	outputStr := output.String()
	c.Assert(outputStr, qt.Contains, "🔍 DRY RUN MODE")
	// Should generate a clean slug from the name with special characters
	c.Assert(outputStr, qt.Contains, "test-organization-with-special-characters")

	t.Log("✅ Slug generation (dry-run) completed successfully")
}
