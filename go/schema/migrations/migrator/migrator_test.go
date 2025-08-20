package migrator_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/schema/migrations/migrator"
)

func TestMigrator_parsePostgreSQLDSN(t *testing.T) {
	tests := []struct {
		name        string
		dbURL       string
		expectedDB  string
		expectedDSN string
		expectError bool
	}{
		{
			name:        "valid PostgreSQL DSN",
			dbURL:       "postgres://user:pass@localhost:5432/testdb?sslmode=disable",
			expectedDB:  "testdb",
			expectedDSN: "postgres://user:pass@localhost:5432/postgres?sslmode=disable",
			expectError: false,
		},
		{
			name:        "valid PostgreSQL DSN with postgresql scheme",
			dbURL:       "postgresql://user:pass@localhost:5432/myapp",
			expectedDB:  "myapp",
			expectedDSN: "postgresql://user:pass@localhost:5432/postgres",
			expectError: false,
		},
		{
			name:        "invalid scheme",
			dbURL:       "mysql://user:pass@localhost:3306/testdb",
			expectedDB:  "",
			expectedDSN: "",
			expectError: true,
		},
		{
			name:        "missing database name",
			dbURL:       "postgres://user:pass@localhost:5432/",
			expectedDB:  "",
			expectedDSN: "",
			expectError: true,
		},
		{
			name:        "invalid URL",
			dbURL:       "not-a-url",
			expectedDB:  "",
			expectedDSN: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Create migrator with test URL
			m := migrator.New(tt.dbURL, nil)

			// Use reflection to access the private method for testing
			// Since parsePostgreSQLDSN is private, we'll test it indirectly through DropDatabase
			// with dry-run mode to avoid actual database operations
			err := m.DropDatabase(context.Background(), true, true) // dryRun=true, confirm=true

			if tt.expectError {
				c.Assert(err, qt.IsNotNil)
			} else {
				c.Assert(err, qt.IsNil)
			}
		})
	}
}

func TestMigrator_DropDatabase_DryRun(t *testing.T) {
	c := qt.New(t)

	// Test with a valid PostgreSQL DSN in dry-run mode
	dbURL := "postgres://user:pass@localhost:5432/testdb?sslmode=disable"
	m := migrator.New(dbURL, nil)

	// Test dry-run mode (should not fail even with invalid connection)
	err := m.DropDatabase(context.Background(), true, true) // dryRun=true, confirm=true
	c.Assert(err, qt.IsNil)
}

func TestMigrator_DropDatabase_InvalidScheme(t *testing.T) {
	c := qt.New(t)

	// Test with invalid database scheme
	dbURL := "mysql://user:pass@localhost:3306/testdb"
	m := migrator.New(dbURL, nil)

	// Should fail even in dry-run mode due to invalid scheme
	err := m.DropDatabase(context.Background(), true, true) // dryRun=true, confirm=true
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "unsupported database scheme")
}
