package shared_test

import (
	"bytes"
	"log/slog"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
)

// captureSlog swaps the default slog logger for one writing to a buffer for the
// duration of the test, returning the buffer. ReadSection logs its cleanenv
// failures (the "Failed to read config" / "unsupported type" error) through the
// default logger and swallows them, so capturing the default logger is the only
// way to assert the error is gone.
func captureSlog(c *qt.C) *bytes.Buffer {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	c.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// ptrBoolSectionFixed mirrors the production bootstrap.Config shape AFTER the
// Fix 2 change: the *bool field carries ONLY a yaml tag (no env tag), and a
// sibling string field keeps an env binding. This locks in that a *bool with no
// env tag does not trip cleanenv's "unsupported type ." default arm while the
// sibling env binding still resolves (precedence preserved).
type ptrBoolSectionFixed struct {
	MFAEnforced *bool  `yaml:"mfa-enforced"`
	Email       string `yaml:"email" env:"EMAIL"`
}

// ptrBoolSectionBuggy mirrors the pre-fix shape: the *bool field carries an env
// tag. This reproduces the bug so the test documents exactly what was wrong.
type ptrBoolSectionBuggy struct {
	MFAEnforced *bool `yaml:"mfa-enforced" env:"MFA_ENFORCED"`
}

// TestReadSection_PtrBoolNoEnvTag_NoError pins Fix 2: reading a section whose
// *bool field has no env tag emits no cleanenv parse error, leaves the pointer
// nil (so the secure-default-true resolution downstream is preserved), and
// still binds the sibling string field from the environment (precedence intact).
func TestReadSection_PtrBoolNoEnvTag_NoError(t *testing.T) {
	c := qt.New(t)
	buf := captureSlog(c)

	t.Setenv("INVENTARIO_BACKOFFICE_BOOTSTRAP_EMAIL", "ops@example.com")
	// Set the env var the buggy binding used to read; with the env tag dropped
	// it must simply be ignored rather than fail to parse.
	t.Setenv("INVENTARIO_BACKOFFICE_BOOTSTRAP_MFA_ENFORCED", "false")

	var section ptrBoolSectionFixed
	err := shared.ReadSection("backoffice.bootstrap", &section)
	c.Assert(err, qt.IsNil)

	// No cleanenv parse error was logged-and-swallowed.
	c.Assert(buf.String(), qt.Not(qt.Contains), "Failed to read config")
	c.Assert(buf.String(), qt.Not(qt.Contains), "unsupported type")

	// The *bool stays nil (no env binding), so resolveMFAEnforced applies the
	// secure default (true) downstream rather than a silent env-driven false.
	c.Assert(section.MFAEnforced, qt.IsNil)
	// The sibling string field still binds from env: the read did not abort.
	c.Assert(section.Email, qt.Equals, "ops@example.com")
}

// TestReadSection_PtrBoolWithEnvTag_LogsError documents the regression Fix 2
// removes: a *bool WITH an env tag falls through cleanenv's default arm and
// logs the "unsupported type ." error on every read. Kept as a guard so a
// future re-add of the env tag is caught by an explicit, named test.
func TestReadSection_PtrBoolWithEnvTag_LogsError(t *testing.T) {
	c := qt.New(t)
	buf := captureSlog(c)

	t.Setenv("INVENTARIO_BACKOFFICE_BOOTSTRAP_MFA_ENFORCED", "false")

	var section ptrBoolSectionBuggy
	err := shared.ReadSection("backoffice.bootstrap", &section)
	// ReadSection swallows the cleanenv error (TryReadSection is non-fatal), so
	// it still returns nil — but the error is logged.
	c.Assert(err, qt.IsNil)
	c.Assert(buf.String(), qt.Contains, "unsupported type")
}
