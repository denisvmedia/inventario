package bootstrap_test

import (
	"bytes"
	"log/slog"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/cmd/inventario/run/bootstrap"
	"go.5x5.cz/inventario/cmd/inventario/shared"
)

// A DSN that does not parse stops the startup with an error, not a panic,
// and neither the error nor the log line carries the password.
func TestBuild_UnparseableDSNIsAnErrorWithoutThePassword(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	c.Cleanup(func() { slog.SetDefault(previous) })

	_, err := bootstrap.Build(&bootstrap.Config{}, &shared.DatabaseConfig{
		DBDSN: "postgres://app:s3cret%zz@db:5432/inventario",
	}, bootstrap.ModeAPIServer)

	c.Assert(err, qt.ErrorMatches, `invalid --db-dsn: invalid URL escape "%zz"`)
	c.Assert(buf.String(), qt.Contains, "Invalid database DSN")
	c.Assert(buf.String(), qt.Not(qt.Contains), "s3cret")
}
