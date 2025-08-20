package registry_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/registry"
)

func TestUserIDFromContext(t *testing.T) {
	testCases := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "user ID present in context",
			ctx:      registry.WithUserContext(context.Background(), "user-123"),
			expected: "user-123",
		},
		{
			name:     "no user ID in context",
			ctx:      context.Background(),
			expected: "",
		},
		{
			name:     "empty user ID in context",
			ctx:      registry.WithUserContext(context.Background(), ""),
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			userID := registry.UserIDFromContext(tc.ctx)
			c.Assert(userID, qt.Equals, tc.expected)
		})
	}
}

func TestValidateUserContext(t *testing.T) {
	testCases := []struct {
		name        string
		ctx         context.Context
		expectError bool
	}{
		{
			name:        "valid user context",
			ctx:         registry.WithUserContext(context.Background(), "user-123"),
			expectError: false,
		},
		{
			name:        "no user context",
			ctx:         context.Background(),
			expectError: true,
		},
		{
			name:        "empty user ID",
			ctx:         registry.WithUserContext(context.Background(), ""),
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			err := registry.ValidateUserContext(tc.ctx)
			if tc.expectError {
				c.Assert(err, qt.IsNotNil)
				c.Assert(err, qt.ErrorMatches, ".*user context required.*")
			} else {
				c.Assert(err, qt.IsNil)
			}
		})
	}
}

func TestRequireUserID(t *testing.T) {
	testCases := []struct {
		name        string
		ctx         context.Context
		expectedID  string
		expectError bool
	}{
		{
			name:        "valid user context",
			ctx:         registry.WithUserContext(context.Background(), "user-123"),
			expectedID:  "user-123",
			expectError: false,
		},
		{
			name:        "no user context",
			ctx:         context.Background(),
			expectedID:  "",
			expectError: true,
		},
		{
			name:        "empty user ID",
			ctx:         registry.WithUserContext(context.Background(), ""),
			expectedID:  "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			userID, err := registry.RequireUserID(tc.ctx)
			if tc.expectError {
				c.Assert(err, qt.IsNotNil)
				c.Assert(err, qt.ErrorMatches, ".*user context required.*")
				c.Assert(userID, qt.Equals, "")
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(userID, qt.Equals, tc.expectedID)
			}
		})
	}
}

func TestUserContextExecutor(t *testing.T) {
	t.Run("execute with valid user ID", func(t *testing.T) {
		c := qt.New(t)

		executor := registry.NewUserContextExecutor("user-123")

		var capturedUserID string
		err := executor.Execute(context.Background(), func(ctx context.Context) error {
			capturedUserID = registry.UserIDFromContext(ctx)
			return nil
		})

		c.Assert(err, qt.IsNil)
		c.Assert(capturedUserID, qt.Equals, "user-123")
	})

	t.Run("execute with empty user ID", func(t *testing.T) {
		c := qt.New(t)

		executor := registry.NewUserContextExecutor("")

		err := executor.Execute(context.Background(), func(ctx context.Context) error {
			return nil
		})

		c.Assert(err, qt.IsNotNil)
		c.Assert(err, qt.ErrorMatches, ".*invalid user context.*")
	})
}

func TestExecuteWithUserID(t *testing.T) {
	t.Run("execute with valid user ID", func(t *testing.T) {
		c := qt.New(t)

		var capturedUserID string
		err := registry.ExecuteWithUserID(context.Background(), "user-456", func(ctx context.Context) error {
			capturedUserID = registry.UserIDFromContext(ctx)
			return nil
		})

		c.Assert(err, qt.IsNil)
		c.Assert(capturedUserID, qt.Equals, "user-456")
	})

	t.Run("execute with empty user ID", func(t *testing.T) {
		c := qt.New(t)

		err := registry.ExecuteWithUserID(context.Background(), "", func(ctx context.Context) error {
			return nil
		})

		c.Assert(err, qt.IsNotNil)
		c.Assert(err, qt.ErrorMatches, ".*invalid user context.*")
	})
}
