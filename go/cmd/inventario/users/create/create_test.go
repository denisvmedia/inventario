package create_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/cmd/inventario/users/create"
	"github.com/denisvmedia/inventario/models"
)

func TestCommand_New(t *testing.T) {
	c := qt.New(t)

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

func TestCommand_DatabaseValidation(t *testing.T) {
	tests := []struct {
		name        string
		dsn         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid postgres dsn",
			dsn:         "postgres://user:pass@localhost/db",
			expectError: false,
		},
		{
			name:        "valid postgresql dsn",
			dsn:         "postgresql://user:pass@localhost/db",
			expectError: false,
		},
		{
			name:        "memory dsn rejected",
			dsn:         "memory://",
			expectError: true,
			errorMsg:    "bootstrap migrations only support PostgreSQL databases",
		},
		{
			name:        "empty dsn",
			dsn:         "",
			expectError: true,
			errorMsg:    "database DSN is required",
		},
		{
			name:        "invalid dsn",
			dsn:         "invalid://test",
			expectError: true,
			errorMsg:    "bootstrap migrations only support PostgreSQL databases",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

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
				"--tenant=test-tenant",
				"--no-interactive",
			})

			err := cobraCmd.Execute()

			if tt.expectError {
				c.Assert(err, qt.IsNotNil)
				c.Assert(err.Error(), qt.Contains, tt.errorMsg)
			} else {
				// For valid DSNs, we expect connection errors since we're not using a real database
				// but the DSN validation should pass
				if err != nil {
					c.Assert(err.Error(), qt.Not(qt.Contains), "bootstrap migrations only support PostgreSQL databases")
					c.Assert(err.Error(), qt.Not(qt.Contains), "user creation is not supported for memory databases")
				}
			}
		})
	}
}

func TestCommand_UserValidation(t *testing.T) {
	tests := []struct {
		name        string
		email       string
		password    string
		userName    string
		role        string
		tenant      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid user",
			email:       "test@example.com",
			password:    "TestPassword123",
			userName:    "Test User",
			role:        "user",
			tenant:      "test-tenant",
			expectError: false,
		},
		{
			name:        "empty email",
			email:       "",
			password:    "TestPassword123",
			tenant:      "test-tenant",
			expectError: true,
			errorMsg:    "email address is required",
		},
		{
			name:        "invalid email format",
			email:       "invalid-email",
			password:    "TestPassword123",
			tenant:      "test-tenant",
			expectError: true,
			errorMsg:    "validation failed",
		},
		{
			name:        "empty tenant",
			email:       "test@example.com",
			password:    "TestPassword123",
			tenant:      "",
			expectError: true,
			errorMsg:    "tenant is required",
		},
		{
			name:        "invalid role",
			email:       "test@example.com",
			password:    "TestPassword123",
			role:        "invalid",
			tenant:      "test-tenant",
			expectError: true,
			errorMsg:    "validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			dbConfig := &shared.DatabaseConfig{
				DBDSN: "postgres://test:test@localhost/test",
			}

			cmd := create.New(dbConfig)
			cobraCmd := cmd.Cmd()

			args := []string{
				"--dry-run",
				"--no-interactive",
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

			if tt.expectError {
				c.Assert(err, qt.IsNotNil)
				c.Assert(err.Error(), qt.Contains, tt.errorMsg)
			} else {
				// For valid inputs, we expect connection errors since we're not using a real database
				if err != nil {
					c.Assert(err.Error(), qt.Not(qt.Contains), "email address is required")
					c.Assert(err.Error(), qt.Not(qt.Contains), "tenant is required")
					c.Assert(err.Error(), qt.Not(qt.Contains), "validation failed")
				}
			}
		})
	}
}

func TestCommand_PasswordValidation(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid password",
			password:    "TestPassword123",
			expectError: false,
		},
		{
			name:        "password too short",
			password:    "Test1",
			expectError: true,
			errorMsg:    "password must be at least 8 characters long",
		},
		{
			name:        "password without uppercase",
			password:    "testpassword123",
			expectError: true,
			errorMsg:    "password must contain at least one uppercase letter",
		},
		{
			name:        "password without lowercase",
			password:    "TESTPASSWORD123",
			expectError: true,
			errorMsg:    "password must contain at least one lowercase letter",
		},
		{
			name:        "password without digit",
			password:    "TestPassword",
			expectError: true,
			errorMsg:    "password must contain at least one digit",
		},
		{
			name:        "empty password in non-interactive mode",
			password:    "",
			expectError: true,
			errorMsg:    "password is required in non-interactive mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			dbConfig := &shared.DatabaseConfig{
				DBDSN: "postgres://test:test@localhost/test",
			}

			cmd := create.New(dbConfig)
			cobraCmd := cmd.Cmd()

			args := []string{
				"--dry-run",
				"--email=test@example.com",
				"--tenant=test-tenant",
				"--no-interactive",
			}

			if tt.password != "" {
				args = append(args, "--password="+tt.password)
			}

			cobraCmd.SetArgs(args)

			err := cobraCmd.Execute()

			if tt.expectError {
				c.Assert(err, qt.IsNotNil)
				c.Assert(err.Error(), qt.Contains, tt.errorMsg)
			} else {
				// For valid inputs, we expect connection errors since we're not using a real database
				if err != nil {
					c.Assert(err.Error(), qt.Not(qt.Contains), "password must be at least 8 characters long")
					c.Assert(err.Error(), qt.Not(qt.Contains), "password must contain at least one uppercase letter")
					c.Assert(err.Error(), qt.Not(qt.Contains), "password must contain at least one lowercase letter")
					c.Assert(err.Error(), qt.Not(qt.Contains), "password must contain at least one digit")
					c.Assert(err.Error(), qt.Not(qt.Contains), "password is required in non-interactive mode")
				}
			}
		})
	}
}

func TestUserRole_Validation(t *testing.T) {
	c := qt.New(t)

	validRoles := []models.UserRole{
		models.UserRoleAdmin,
		models.UserRoleUser,
	}

	for _, role := range validRoles {
		err := role.Validate()
		c.Assert(err, qt.IsNil, qt.Commentf("Role %s should be valid", role))
	}

	invalidRole := models.UserRole("invalid")
	err := invalidRole.Validate()
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "must be one of: admin, user")
}

func TestPasswordValidation_Function(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid password",
			password:    "TestPassword123",
			expectError: false,
		},
		{
			name:        "password too short",
			password:    "Test1",
			expectError: true,
			errorMsg:    "password must be at least 8 characters long",
		},
		{
			name:        "password without uppercase",
			password:    "testpassword123",
			expectError: true,
			errorMsg:    "password must contain at least one uppercase letter",
		},
		{
			name:        "password without lowercase",
			password:    "TESTPASSWORD123",
			expectError: true,
			errorMsg:    "password must contain at least one lowercase letter",
		},
		{
			name:        "password without digit",
			password:    "TestPassword",
			expectError: true,
			errorMsg:    "password must contain at least one digit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			err := models.ValidatePassword(tt.password)

			if tt.expectError {
				c.Assert(err, qt.IsNotNil)
				c.Assert(err.Error(), qt.Contains, tt.errorMsg)
			} else {
				c.Assert(err, qt.IsNil)
			}
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
