package filesbackfill

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

// redactDSN must not let passwords reach stdout, must keep host/db/query
// readable for ops, and must not silently mangle non-URL DSNs.
func TestRedactDSN(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "postgres:// with password",
			dsn:  "postgres://inventario:s3cret@localhost:5432/inventario?sslmode=disable",
			want: "postgres://inventario:***@localhost:5432/inventario?sslmode=disable",
		},
		{
			name: "postgresql:// alias",
			dsn:  "postgresql://user:hunter2@db.internal:5432/app",
			want: "postgresql://user:***@db.internal:5432/app",
		},
		{
			name: "no userinfo at all",
			dsn:  "postgres://localhost:5432/inventario",
			want: "postgres://localhost:5432/inventario",
		},
		{
			name: "user without password",
			dsn:  "postgres://inventario@localhost:5432/inventario",
			want: "postgres://inventario@localhost:5432/inventario",
		},
		{
			name: "memory:// scheme passes through",
			dsn:  "memory://",
			want: "memory://",
		},
		{
			name: "malformed input passes through",
			dsn:  "not a url",
			want: "not a url",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(redactDSN(tc.dsn), qt.Equals, tc.want)
		})
	}
}
