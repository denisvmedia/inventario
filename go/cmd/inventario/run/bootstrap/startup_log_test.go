package bootstrap_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/cmd/inventario/run/bootstrap"
	"go.5x5.cz/inventario/cmd/inventario/shared"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

// captureStartupLogs points the default logger at a buffer until the test
// ends. The tests that call it must not run in parallel: the default logger
// is process state.
func captureStartupLogs(c *qt.C) *bytes.Buffer {
	c.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	c.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}

// validStartupConfig is a configuration that starts against the memory
// database, so each test changes the one setting it is about.
func validStartupConfig() *bootstrap.Config {
	cfg := &bootstrap.Config{FileURLExpiration: "15m", EmailProvider: "stub"}
	cfg.SetDefaults()
	return cfg
}

var memoryDB = &shared.DatabaseConfig{DBDSN: "memory://"}

// registerMemoryDB makes memory:// resolvable, as main does, until the test
// ends.
func registerMemoryDB(c *qt.C) {
	c.Helper()
	cleanup := memory.Register()
	c.Cleanup(func() {
		registry.Unregister(memory.Name)
		_ = cleanup()
	})
}

// A schema check that fails for a reason other than a lagging schema, here a
// database nothing listens for, stops the startup with one line, and the line
// leaves the password out.
func TestBuild_SchemaCheckFailureIsLoggedOnce(t *testing.T) {
	c := qt.New(t)
	buf := captureStartupLogs(c)

	_, err := bootstrap.Build(validStartupConfig(), &shared.DatabaseConfig{
		DBDSN: "postgres://app:s3cret@127.0.0.1:1/inventario?connect_timeout=2",
	}, bootstrap.ModeAPIServer)

	c.Assert(err, qt.IsNotNil)
	c.Assert(strings.Count(buf.String(), "Refusing to start: could not check the database schema"), qt.Equals, 1)
	c.Assert(buf.String(), qt.Not(qt.Contains), "s3cret")
}

// The stub email provider with a public URL set is refused at startup, and
// the refusal is logged, not only returned to the command line.
func TestBuild_EmailPublicURLConfigFailureIsLogged(t *testing.T) {
	c := qt.New(t)
	buf := captureStartupLogs(c)
	registerMemoryDB(c)
	cfg := validStartupConfig()
	cfg.PublicURL = "https://inventario.example.com"

	_, err := bootstrap.Build(cfg, memoryDB, bootstrap.ModeAPIServer)

	c.Assert(err, qt.ErrorMatches, `email provider is "stub" but a public URL is set .*`)
	c.Assert(buf.String(), qt.Contains, "Invalid email or public URL configuration")
}

// A Sentry setting that does not parse stops the startup with a line.
func TestBuild_SentryConfigFailureIsLogged(t *testing.T) {
	c := qt.New(t)
	t.Setenv("SENTRY_TRACES_SAMPLE_RATE", "often")
	registerMemoryDB(c)
	buf := captureStartupLogs(c)

	_, err := bootstrap.Build(validStartupConfig(), memoryDB, bootstrap.ModeAPIServer)

	c.Assert(err, qt.ErrorMatches, `loading sentry config: .*`)
	c.Assert(buf.String(), qt.Contains, "Invalid Sentry configuration")
}

// A Sentry DSN that does not parse stops the startup with a line that leaves
// the DSN, and the secret key it can carry, out.
func TestBuild_SentryDSNFailureIsLoggedWithoutTheDSN(t *testing.T) {
	c := qt.New(t)
	t.Setenv("SENTRY_DSN", "https://public:s3cret#key@o1.ingest.sentry.io/1")
	registerMemoryDB(c)
	buf := captureStartupLogs(c)

	_, err := bootstrap.Build(validStartupConfig(), memoryDB, bootstrap.ModeAPIServer)

	c.Assert(err, qt.ErrorMatches, `initialising sentry: invalid SENTRY_DSN: .*`)
	c.Assert(buf.String(), qt.Contains, "Failed to initialize Sentry")
	c.Assert(buf.String(), qt.Not(qt.Contains), "s3cret")
	c.Assert(err.Error(), qt.Not(qt.Contains), "s3cret")
}
