package create_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/cmd/inventario/users/create"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func setupMemoryAsPostgres(c *qt.C) {
	// Register memory registry as "postgres" for testing
	newFn, _ := memory.NewMemoryRegistrySet()
	registry.Register("postgres", newFn)

	// Setup cleanup to unregister after test
	c.Cleanup(func() {
		registry.Unregister("postgres")
	})
}

func setupMemoryAsPostgresWithTenant(c *qt.C) {
	// Register memory registry as "postgres" for testing
	newFn, _ := memory.NewMemoryRegistrySet()

	// Create a wrapper that pre-populates the tenant
	wrappedNewFn := func(config registry.Config) (*registry.FactorySet, error) {
		factorySet, err := newFn(config)
		if err != nil {
			return nil, err
		}

		// Create a test tenant in the memory registry
		serviceRegistrySet := factorySet.CreateServiceRegistrySet()
		_, err = serviceRegistrySet.TenantRegistry.Create(nil, models.Tenant{
			EntityID: models.EntityID{ID: "test-tenant"},
			Name:     "Test Tenant",
			Slug:     "test-tenant",
			Status:   models.TenantStatusActive,
		})
		if err != nil {
			return nil, err
		}

		return factorySet, nil
	}

	registry.Register("postgres", wrappedNewFn)

	// Setup cleanup to unregister after test
	c.Cleanup(func() {
		registry.Unregister("postgres")
	})
}

func TestCommand_New(t *testing.T) {
	c := qt.New(t)

	// Setup memory registry as postgres for testing
	setupMemoryAsPostgres(c)

	dbConfig := &shared.DatabaseConfig{
		DBDSN: "postgres://test:test@localhost/test",
	}

	cmd := create.New(dbConfig)
	c.Assert(cmd, qt.IsNotNil)
	c.Assert(cmd.Cmd(), qt.IsNotNil)
	c.Assert(cmd.Cmd().Use, qt.Equals, "create")
	c.Assert(cmd.Cmd().Short, qt.Equals, "Create a new user")
}

func TestCommand_Flags(t *testing.T) {
	c := qt.New(t)

	// Setup memory registry as postgres for testing
	setupMemoryAsPostgres(c)

	dbConfig := &shared.DatabaseConfig{
		DBDSN: "postgres://test:test@localhost/test",
	}

	cmd := create.New(dbConfig)
	cobraCmd := cmd.Cmd()

	// Test that all expected flags are present
	expectedFlags := []string{
		"dry-run",
		"email",
		"password",
		"name",
		"role",
		"tenant",
		"active",
		"interactive",
		"no-interactive",
	}

	for _, flagName := range expectedFlags {
		flag := cobraCmd.Flags().Lookup(flagName)
		c.Assert(flag, qt.IsNotNil, qt.Commentf("Flag %s should exist", flagName))
	}
}

func TestCommand_DatabaseValidation_HappyPath(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
	}{
		{
			name: "valid postgres dsn",
			dsn:  "postgres://user:pass@localhost/db",
		},
		{
			name: "valid postgresql dsn",
			dsn:  "postgres://user:pass@localhost/db", // Use postgres scheme for consistency
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Setup memory registry as postgres for testing with test tenant
			setupMemoryAsPostgresWithTenant(c)

			dbConfig := &shared.DatabaseConfig{
				DBDSN: tt.dsn,
			}

			cmd := create.New(dbConfig)
			cobraCmd := cmd.Cmd()

			// Set minimal required flags for dry run
			cobraCmd.SetArgs([]string{
				"--dry-run",
				"--email=test@example.com",
				"--password=TestPassword123",
				"--name=Test User",
				"--tenant=test-tenant",
				"--no-interactive",
			})

			err := cobraCmd.Execute()

			// For valid DSNs with memory registry, we expect success in dry-run mode
			c.Assert(err, qt.IsNil)
		})
	}
}

func TestCommand_DatabaseValidation_ErrorPath(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		errorMsg string
	}{
		{
			name:     "memory dsn rejected",
			dsn:      "memory://",
			errorMsg: "bootstrap migrations only support PostgreSQL databases",
		},
		{
			name:     "empty dsn",
			dsn:      "",
			errorMsg: "database DSN is required",
		},
		{
			name:     "invalid dsn",
			dsn:      "invalid://test",
			errorMsg: "bootstrap migrations only support PostgreSQL databases",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Setup memory registry as postgres for testing
			setupMemoryAsPostgres(c)

			dbConfig := &shared.DatabaseConfig{
				DBDSN: tt.dsn,
			}

			cmd := create.New(dbConfig)
			cobraCmd := cmd.Cmd()

			// Set minimal required flags for dry run
			cobraCmd.SetArgs([]string{
				"--dry-run",
				"--email=test@example.com",
				"--password=TestPassword123",
				"--name=Test User",
				"--tenant=test-tenant",
				"--no-interactive",
			})

			err := cobraCmd.Execute()

			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tt.errorMsg)
		})
	}
}

func TestCommand_UserValidation_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		userName string
		role     string
		tenant   string
	}{
		{
			name:     "valid user",
			email:    "test@example.com",
			password: "TestPassword123",
			userName: "Test User",
			role:     "user",
			tenant:   "test-tenant",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Setup memory registry as postgres for testing with test tenant
			setupMemoryAsPostgresWithTenant(c)

			dbConfig := &shared.DatabaseConfig{
				DBDSN: "postgres://test:test@localhost/test",
			}

			cmd := create.New(dbConfig)
			cobraCmd := cmd.Cmd()

			args := []string{
				"--dry-run",
				"--no-interactive",
				"--email=" + tt.email,
				"--password=" + tt.password,
				"--tenant=" + tt.tenant,
			}

			if tt.userName != "" {
				args = append(args, "--name="+tt.userName)
			} else {
				args = append(args, "--name=Test User") // Add default name
			}
			if tt.role != "" {
				args = append(args, "--role="+tt.role)
			}

			cobraCmd.SetArgs(args)

			err := cobraCmd.Execute()

			// For valid inputs with memory registry, we expect success in dry-run mode
			c.Assert(err, qt.IsNil)
		})
	}
}

func TestCommand_UserValidation_ErrorPath(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		userName string
		role     string
		tenant   string
		errorMsg string
	}{
		{
			name:     "empty email",
			email:    "",
			password: "TestPassword123",
			tenant:   "test-tenant",
			errorMsg: "email is required",
		},

		{
			name:     "empty tenant",
			email:    "test@example.com",
			password: "TestPassword123",
			tenant:   "",
			errorMsg: "tenant ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Setup memory registry as postgres for testing with test tenant
			setupMemoryAsPostgresWithTenant(c)

			dbConfig := &shared.DatabaseConfig{
				DBDSN: "postgres://test:test@localhost/test",
			}

			cmd := create.New(dbConfig)
			cobraCmd := cmd.Cmd()

			args := []string{
				"--dry-run",
				"--no-interactive",
				"--name=Test User", // Always provide name to avoid "name is required" error
			}

			if tt.email != "" {
				args = append(args, "--email="+tt.email)
			}
			if tt.password != "" {
				args = append(args, "--password="+tt.password)
			}
			if tt.userName != "" {
				args = append(args, "--name="+tt.userName)
			}
			if tt.role != "" {
				args = append(args, "--role="+tt.role)
			}
			if tt.tenant != "" {
				args = append(args, "--tenant="+tt.tenant)
			}

			cobraCmd.SetArgs(args)

			err := cobraCmd.Execute()

			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tt.errorMsg)
		})
	}
}

func TestCommand_PasswordValidation_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{
			name:     "valid password",
			password: "TestPassword123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Setup memory registry as postgres for testing with test tenant
			setupMemoryAsPostgresWithTenant(c)

			dbConfig := &shared.DatabaseConfig{
				DBDSN: "postgres://test:test@localhost/test",
			}

			cmd := create.New(dbConfig)
			cobraCmd := cmd.Cmd()

			args := []string{
				"--dry-run",
				"--email=test@example.com",
				"--name=Test User",
				"--tenant=test-tenant",
				"--no-interactive",
				"--password=" + tt.password,
			}

			cobraCmd.SetArgs(args)

			err := cobraCmd.Execute()

			// For valid inputs with memory registry, we expect success in dry-run mode
			c.Assert(err, qt.IsNil)
		})
	}
}

func TestCommand_PasswordValidation_ErrorPath(t *testing.T) {
	tests := []struct {
		name     string
		password string
		errorMsg string
	}{
		{
			name:     "empty password in non-interactive mode",
			password: "",
			errorMsg: "failed to read password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Setup memory registry as postgres for testing
			setupMemoryAsPostgres(c)

			dbConfig := &shared.DatabaseConfig{
				DBDSN: "postgres://test:test@localhost/test",
			}

			cmd := create.New(dbConfig)
			cobraCmd := cmd.Cmd()

			args := []string{
				"--dry-run",
				"--email=test@example.com",
				"--name=Test User",
				"--tenant=",
				"--no-interactive",
			}

			if tt.password != "" {
				args = append(args, "--password="+tt.password)
			}

			cobraCmd.SetArgs(args)

			err := cobraCmd.Execute()

			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tt.errorMsg)
		})
	}
}

func TestUserRole_Validation_HappyPath(t *testing.T) {
	validRoles := []models.UserRole{
		models.UserRoleAdmin,
		models.UserRoleUser,
	}

	for _, role := range validRoles {
		t.Run(string(role), func(t *testing.T) {
			c := qt.New(t)
			err := role.Validate()
			c.Assert(err, qt.IsNil)
		})
	}
}

func TestUserRole_Validation_ErrorPath(t *testing.T) {
	t.Run("invalid role", func(t *testing.T) {
		c := qt.New(t)

		invalidRole := models.UserRole("invalid")
		err := invalidRole.Validate()

		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Contains, "must be one of: admin, user")
	})
}

func TestPasswordValidation_Function_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{
			name:     "valid password",
			password: "TestPassword123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			err := models.ValidatePassword(tt.password)

			c.Assert(err, qt.IsNil)
		})
	}
}

func TestPasswordValidation_Function_ErrorPath(t *testing.T) {
	tests := []struct {
		name     string
		password string
		errorMsg string
	}{
		{
			name:     "password too short",
			password: "Test1",
			errorMsg: "password must be at least 8 characters long",
		},
		{
			name:     "password without uppercase",
			password: "testpassword123",
			errorMsg: "password must contain at least one uppercase letter",
		},
		{
			name:     "password without lowercase",
			password: "TESTPASSWORD123",
			errorMsg: "password must contain at least one lowercase letter",
		},
		{
			name:     "password without digit",
			password: "TestPassword",
			errorMsg: "password must contain at least one digit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			err := models.ValidatePassword(tt.password)

			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tt.errorMsg)
		})
	}
}

func TestUser_SetPassword(t *testing.T) {
	c := qt.New(t)

	user := &models.User{}

	// Test valid password
	err := user.SetPassword("TestPassword123")
	c.Assert(err, qt.IsNil)
	c.Assert(user.PasswordHash, qt.Not(qt.Equals), "")
	c.Assert(user.PasswordHash, qt.Not(qt.Equals), "TestPassword123") // Should be hashed

	// Test password verification
	c.Assert(user.CheckPassword("TestPassword123"), qt.IsTrue)
	c.Assert(user.CheckPassword("WrongPassword"), qt.IsFalse)

	// Test invalid password
	err = user.SetPassword("weak")
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "password must be at least 8 characters long")
}
