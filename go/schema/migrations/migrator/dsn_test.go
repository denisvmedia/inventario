package migrator_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"testing/fstest"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/schema/migrations/migrator"
)

// lib/pq parses the DSN on first use and returns the url.Parse error, which
// quotes the DSN with its password. The migrator returns the reason only.
func TestVerifySchemaUpToDate_UnparseableDSNErrorLeavesThePasswordOut(t *testing.T) {
	c := qt.New(t)

	m := migrator.New("postgres://app:s3cret%zz@db:5432/inventario", fstest.MapFS{})
	err := m.VerifySchemaUpToDate(context.Background())

	c.Assert(err, qt.ErrorMatches, `.*invalid URL escape "%zz".*`)
	c.Assert(err.Error(), qt.Not(qt.Contains), "s3cret")
}

// The line `db migrate up` writes on every run names the database without
// its password.
func TestMigrateUp_LogLeavesThePasswordOut(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	// Nothing listens on port 1, so the run stops at the connection, after
	// the line that names the database.
	m := migrator.New("postgres://app:s3cret@127.0.0.1:1/inventario?connect_timeout=2", fstest.MapFS{}).SetLogger(logger)

	err := m.MigrateUp(context.Background(), migrator.Args{})

	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Not(qt.Contains), "s3cret")
	c.Assert(buf.String(), qt.Contains, "Applying migrations up")
	c.Assert(buf.String(), qt.Contains, "127.0.0.1:1/inventario")
	c.Assert(buf.String(), qt.Not(qt.Contains), "s3cret")
}
