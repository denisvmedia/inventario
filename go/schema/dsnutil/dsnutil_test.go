package dsnutil_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/schema/dsnutil"
)

func TestStripPGXPoolParams(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "removes pool_max_conns and pool_min_conns",
			input:    "postgres://user:pass@localhost:5432/db?sslmode=disable&pool_max_conns=1&pool_min_conns=1",
			expected: "postgres://user:pass@localhost:5432/db?sslmode=disable",
		},
		{
			name:     "removes other pool_ prefixed params",
			input:    "postgres://user:pass@localhost:5432/db?pool_max_conn_lifetime=1h&pool_health_check_period=30s",
			expected: "postgres://user:pass@localhost:5432/db",
		},
		{
			name:     "preserves DSN with no pool_ params",
			input:    "postgres://user:pass@localhost:5432/db?sslmode=disable",
			expected: "postgres://user:pass@localhost:5432/db?sslmode=disable",
		},
		{
			name:     "preserves DSN with no query string",
			input:    "postgres://user:pass@localhost:5432/db",
			expected: "postgres://user:pass@localhost:5432/db",
		},
		{
			name:     "preserves non-pool params alongside stripped pool params",
			input:    "postgres://u:p@h:5432/db?sslmode=disable&application_name=app&pool_max_conns=4",
			expected: "postgres://u:p@h:5432/db?application_name=app&sslmode=disable",
		},
		{
			name:     "preserves postgresql scheme",
			input:    "postgresql://u:p@h:5432/db?sslmode=disable&pool_max_conns=4",
			expected: "postgresql://u:p@h:5432/db?sslmode=disable",
		},
		{
			name:     "returns input unchanged when URL is unparseable",
			input:    "::not-a-url",
			expected: "::not-a-url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			got := dsnutil.StripPGXPoolParams(tt.input)
			c.Assert(got, qt.Equals, tt.expected)
		})
	}
}

func TestRedact(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "masks the password",
			input:    "postgres://user:s3cr3t@localhost:5432/inventario?sslmode=disable",
			expected: "postgres://user:xxxxxx@localhost:5432/inventario?sslmode=disable",
		},
		{
			name:     "keeps username, host, database and params",
			input:    "postgresql://admin:hunter2@db.internal:6543/app?pool_max_conns=10",
			expected: "postgresql://admin:xxxxxx@db.internal:6543/app?pool_max_conns=10",
		},
		{
			name:     "userinfo without a password is returned unchanged",
			input:    "postgres://user@localhost/inventario",
			expected: "postgres://user@localhost/inventario",
		},
		{
			name:     "no userinfo is returned unchanged",
			input:    "memory://",
			expected: "memory://",
		},
		{
			// A control character makes url.Parse fail; the raw value (which
			// could embed credentials) must not be echoed back.
			name:     "an unparseable DSN never echoes the raw value",
			input:    "postgres://user:s3cr3t@localhost/db\x7f",
			expected: "<redacted>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			c.Assert(dsnutil.Redact(tt.input), qt.Equals, tt.expected)
		})
	}
}
