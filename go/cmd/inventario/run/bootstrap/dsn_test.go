package bootstrap_test

import (
	"bytes"
	"log/slog"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/cmd/inventario/run/bootstrap"
	"go.5x5.cz/inventario/cmd/inventario/shared"
	"go.5x5.cz/inventario/internal/urlerr"
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
		DBDSN: "postgres://app:s3cret#tail@db:5432/inventario",
	}, bootstrap.ModeAPIServer)

	var urlErr *urlerr.Error
	c.Assert(err, qt.ErrorAs, &urlErr)
	c.Assert(urlErr.Reason, qt.Equals, "invalid port after host")
	c.Assert(err, qt.ErrorMatches, `invalid --db-dsn: invalid port after host \(.*\)`)
	c.Assert(err.Error(), qt.Not(qt.Contains), "s3cret")
	c.Assert(buf.String(), qt.Contains, "Invalid database DSN")
	c.Assert(buf.String(), qt.Not(qt.Contains), "s3cret")
}

// The startup banner masks a password given as a query parameter too.
func TestBuild_BannerMasksAPasswordQueryParameter(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	c.Cleanup(func() { slog.SetDefault(previous) })

	// Nothing listens on port 1, so the startup stops at the schema check,
	// after the banner.
	_, err := bootstrap.Build(&bootstrap.Config{}, &shared.DatabaseConfig{
		DBDSN: "postgres://app@127.0.0.1:1/inventario?password=s3cret&connect_timeout=2",
	}, bootstrap.ModeAPIServer)

	c.Assert(err, qt.IsNotNil)
	c.Assert(buf.String(), qt.Contains, "Starting API server")
	c.Assert(buf.String(), qt.Contains, "password=xxxxxx")
	c.Assert(buf.String(), qt.Not(qt.Contains), "s3cret")
}
