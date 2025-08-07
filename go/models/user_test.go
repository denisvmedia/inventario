package models_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestUserRole_Validate(t *testing.T) {
	// Happy path tests
	t.Run("valid user roles", func(t *testing.T) {
		testCases := []struct {
			name string
			role models.UserRole
		}{
			{
				name: "admin role",
				role: models.UserRoleAdmin,
			},
			{
				name: "user role",
				role: models.UserRoleUser,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				c := qt.New(t)
				err := tc.role.Validate()
				c.Assert(err, qt.IsNil)
			})
		}
	})

	// Unhappy path tests
	t.Run("invalid user role", func(t *testing.T) {
		c := qt.New(t)
		invalidRole := models.UserRole("invalid")
		err := invalidRole.Validate()
		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Contains, "must be one of: admin, user")
	})
}

func TestUser_ValidateWithContext(t *testing.T) {
	// Happy path tests
	t.Run("valid user", func(t *testing.T) {
		c := qt.New(t)
		user := &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				TenantID: "tenant-123",
			},
			Email: "test@example.com",
			Name:  "Test User",
			Role:  models.UserRoleUser,
		}

		err := user.ValidateWithContext(context.Background())
		c.Assert(err, qt.IsNil)
	})

	t.Run("valid admin user", func(t *testing.T) {
		c := qt.New(t)
		user := &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				TenantID: "tenant-123",
			},
			Email: "admin@example.com",
			Name:  "Admin User",
			Role:  models.UserRoleAdmin,
		}

		err := user.ValidateWithContext(context.Background())
		c.Assert(err, qt.IsNil)
	})

	// Unhappy path tests
	t.Run("invalid user cases", func(t *testing.T) {
		testCases := []struct {
			name        string
			user        *models.User
			expectedErr string
		}{
			{
				name: "empty email",
				user: &models.User{
					TenantAwareEntityID: models.TenantAwareEntityID{
						TenantID: "tenant-123",
					},
					Email: "",
					Name:  "Test User",
					Role:  models.UserRoleUser,
				},
				expectedErr: "cannot be blank",
			},
			{
				name: "invalid email format",
				user: &models.User{
					TenantAwareEntityID: models.TenantAwareEntityID{
						TenantID: "tenant-123",
					},
					Email: "invalid-email",
					Name:  "Test User",
					Role:  models.UserRoleUser,
				},
				expectedErr: "must be in a valid format",
			},
			{
				name: "empty name",
				user: &models.User{
					TenantAwareEntityID: models.TenantAwareEntityID{
						TenantID: "tenant-123",
					},
					Email: "test@example.com",
					Name:  "",
					Role:  models.UserRoleUser,
				},
				expectedErr: "cannot be blank",
			},
			{
				name: "empty tenant ID",
				user: &models.User{
					Email: "test@example.com",
					Name:  "Test User",
					Role:  models.UserRoleUser,
				},
				expectedErr: "cannot be blank",
			},
			{
				name: "name too long",
				user: &models.User{
					TenantAwareEntityID: models.TenantAwareEntityID{
						TenantID: "tenant-123",
					},
					Email: "test@example.com",
					Name:  "This is a very long user name that exceeds the maximum allowed length of 100 characters for testing purposes",
					Role:  models.UserRoleUser,
				},
				expectedErr: "the length must be between 1 and 100",
			},
			{
				name: "email too long",
				user: &models.User{
					TenantAwareEntityID: models.TenantAwareEntityID{
						TenantID: "tenant-123",
					},
					Email: "this-is-a-very-long-email-address-that-exceeds-the-maximum-allowed-length-of-255-characters-for-testing-purposes-and-should-fail-validation-because-it-is-way-too-long-for-an-email-address-in-any-practical-scenario-that-we-might-encounter-in-real-world-usage-and-this-should-definitely-be-over-255-characters@example.com",
					Name:  "Test User",
					Role:  models.UserRoleUser,
				},
				expectedErr: "the length must be between 1 and 255",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				c := qt.New(t)
				err := tc.user.ValidateWithContext(context.Background())
				c.Assert(err, qt.IsNotNil)
				c.Assert(err.Error(), qt.Contains, tc.expectedErr)
			})
		}
	})
}

func TestUser_SetPassword(t *testing.T) {
	// Happy path tests
	t.Run("set valid password", func(t *testing.T) {
		c := qt.New(t)
		user := &models.User{}
		err := user.SetPassword("ValidPassword123")
		c.Assert(err, qt.IsNil)
		c.Assert(user.PasswordHash, qt.Not(qt.Equals), "")
		c.Assert(user.PasswordHash, qt.Not(qt.Equals), "ValidPassword123")
	})

	// Unhappy path tests
	t.Run("password too short", func(t *testing.T) {
		c := qt.New(t)
		user := &models.User{}
		err := user.SetPassword("short")
		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Contains, "password must be at least 8 characters long")
	})
}

func TestUser_CheckPassword(t *testing.T) {
	// Happy path tests
	t.Run("correct password", func(t *testing.T) {
		c := qt.New(t)
		user := &models.User{}
		password := "ValidPassword123"
		err := user.SetPassword(password)
		c.Assert(err, qt.IsNil)

		isValid := user.CheckPassword(password)
		c.Assert(isValid, qt.IsTrue)
	})

	// Unhappy path tests
	t.Run("incorrect password", func(t *testing.T) {
		c := qt.New(t)
		user := &models.User{}
		err := user.SetPassword("ValidPassword123")
		c.Assert(err, qt.IsNil)

		isValid := user.CheckPassword("WrongPassword")
		c.Assert(isValid, qt.IsFalse)
	})
}

func TestValidatePassword(t *testing.T) {
	// Happy path tests
	t.Run("valid passwords", func(t *testing.T) {
		testCases := []struct {
			name     string
			password string
		}{
			{
				name:     "valid password with all requirements",
				password: "ValidPassword123",
			},
			{
				name:     "minimum length with all requirements",
				password: "Valid1Aa",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				c := qt.New(t)
				err := models.ValidatePassword(tc.password)
				c.Assert(err, qt.IsNil)
			})
		}
	})

	// Unhappy path tests
	t.Run("invalid passwords", func(t *testing.T) {
		testCases := []struct {
			name        string
			password    string
			expectedErr string
		}{
			{
				name:        "too short",
				password:    "Short1",
				expectedErr: "password must be at least 8 characters long",
			},
			{
				name:        "no uppercase letter",
				password:    "validpassword123",
				expectedErr: "password must contain at least one uppercase letter",
			},
			{
				name:        "no lowercase letter",
				password:    "VALIDPASSWORD123",
				expectedErr: "password must contain at least one lowercase letter",
			},
			{
				name:        "no digit",
				password:    "ValidPassword",
				expectedErr: "password must contain at least one digit",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				c := qt.New(t)
				err := models.ValidatePassword(tc.password)
				c.Assert(err, qt.IsNotNil)
				c.Assert(err.Error(), qt.Contains, tc.expectedErr)
			})
		}
	})
}

func TestUser_UpdateLastLogin(t *testing.T) {
	t.Run("update last login timestamp", func(t *testing.T) {
		c := qt.New(t)
		user := &models.User{}
		c.Assert(user.LastLoginAt, qt.IsNil)

		user.UpdateLastLogin()
		c.Assert(user.LastLoginAt, qt.IsNotNil)
	})
}
