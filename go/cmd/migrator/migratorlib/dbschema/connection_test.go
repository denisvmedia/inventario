package dbschema

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestFormatDatabaseURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "PostgreSQL URL with password",
			input:    "postgres://user:secret123@localhost:5432/mydb",
			expected: "postgres://user:***@localhost:5432/mydb",
		},
		{
			name:     "PostgreSQL URL without password",
			input:    "postgres://user@localhost:5432/mydb",
			expected: "postgres://user@localhost:5432/mydb",
		},
		{
			name:     "Invalid URL",
			input:    "not-a-url",
			expected: "not-a-url",
		},
		{
			name:     "MySQL URL with password",
			input:    "mysql://root:password@localhost:3306/testdb",
			expected: "mysql://root:***@localhost:3306/testdb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			result := FormatDatabaseURL(tt.input)
			c.Assert(result, qt.Equals, tt.expected)
		})
	}
}

func TestConnectToDatabase_InvalidURL(t *testing.T) {
	tests := []struct {
		name   string
		dbURL  string
		errMsg string
	}{
		{
			name:   "Invalid URL format",
			dbURL:  "not-a-url",
			errMsg: "invalid database URL: missing scheme",
		},
		{
			name:   "Unsupported dialect",
			dbURL:  "sqlite://test.db",
			errMsg: "unsupported database dialect: sqlite",
		},
		{
			name:   "Empty URL",
			dbURL:  "",
			errMsg: "invalid database URL: missing scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			conn, err := ConnectToDatabase(tt.dbURL)
			c.Assert(err, qt.ErrorMatches, ".*"+tt.errMsg+".*")
			c.Assert(conn, qt.IsNil)
		})
	}
}

func TestConnectToDatabase_UnsupportedDialects(t *testing.T) {
	tests := []struct {
		name     string
		dbURL    string
		expected string
	}{
		{
			name:     "SQLite not supported",
			dbURL:    "sqlite://test.db",
			expected: "unsupported database dialect: sqlite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			conn, err := ConnectToDatabase(tt.dbURL)
			c.Assert(err, qt.ErrorMatches, ".*"+tt.expected+".*")
			c.Assert(conn, qt.IsNil)
		})
	}
}

// TestPostgreSQLConnection tests PostgreSQL connection (will fail if no server running)
func TestPostgreSQLConnection_NoServer(t *testing.T) {
	c := qt.New(t)

	// This test expects to fail since we don't have a PostgreSQL server running
	// It's mainly to test that the connection logic works correctly
	conn, err := ConnectToDatabase("postgres://user:pass@localhost:5432/testdb")

	// We expect an error because no PostgreSQL server is running
	c.Assert(err, qt.IsNotNil)
	c.Assert(conn, qt.IsNil)

	// The error should be about connection failure, not about invalid URL or unsupported dialect
	c.Assert(err.Error(), qt.Not(qt.Contains), "unsupported database dialect")
	c.Assert(err.Error(), qt.Not(qt.Contains), "invalid database URL")
}
