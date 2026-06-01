package bootstrap_test

import (
	"bytes"
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/backoffice/bootstrap"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// setupMemoryAsPostgres registers the in-memory registry under the
// `postgres` scheme so the bootstrap command — which routes through
// `services/backoffice`, which routes through `registry.GetRegistry` —
// resolves a working factory set without an actual postgres instance.
// Mirrors the helper in cmd/inventario/users/create/create_test.go.
//
// Returns the factory set so the test can introspect post-conditions
// (was the row inserted? what's the current count?).
func setupMemoryAsPostgres(c *qt.C) *registry.FactorySet {
	var captured *registry.FactorySet
	newFn, _ := memory.NewMemoryRegistrySet()
	wrappedNewFn := func(cfg registry.Config) (*registry.FactorySet, error) {
		if captured != nil {
			return captured, nil
		}
		fs, err := newFn(cfg)
		if err != nil {
			return nil, err
		}
		captured = fs
		return fs, nil
	}
	registry.Register("postgres", wrappedNewFn)
	c.Cleanup(func() {
		registry.Unregister("postgres")
	})

	// Pre-resolve the factory set so the caller has access to it
	// regardless of whether the command run reaches the registry layer.
	fs, err := wrappedNewFn(registry.Config("postgres://test"))
	c.Assert(err, qt.IsNil)
	return fs
}

// runBootstrap is a one-liner around the cobra command — captures
// stdout for assertion and returns the run error.
func runBootstrap(c *qt.C, dsn string, args ...string) (string, error) {
	dbConfig := &shared.DatabaseConfig{DBDSN: dsn}
	cmd := bootstrap.New(dbConfig).Cmd()

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// TestBootstrap_HappyPath pins the canonical create-with-auto-password
// flow: no rows in the table → command succeeds, prints the generated
// password to stdout, and inserts a single platform_admin row.
func TestBootstrap_HappyPath(t *testing.T) {
	c := qt.New(t)
	fs := setupMemoryAsPostgres(c)

	out, err := runBootstrap(c, "postgres://test",
		"--email=admin@example.com",
		"--name=Ops Admin",
	)
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "Created back-office user")
	c.Assert(out, qt.Contains, "Generated password")

	users, err := fs.BackofficeUserRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 1)
	c.Assert(users[0].Email, qt.Equals, "admin@example.com")
	c.Assert(string(users[0].Role), qt.Equals, "platform_admin")
	c.Assert(users[0].PasswordHash, qt.Not(qt.Equals), "")
}

// TestBootstrap_ExplicitPassword pins that --password skips the
// auto-generation path and DOES NOT print the password.
func TestBootstrap_ExplicitPassword(t *testing.T) {
	c := qt.New(t)
	fs := setupMemoryAsPostgres(c)

	out, err := runBootstrap(c, "postgres://test",
		"--email=admin@example.com",
		"--name=Ops Admin",
		"--password=S3curePass!",
	)
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "Created back-office user")
	c.Assert(out, qt.Not(qt.Contains), "Generated password",
		qt.Commentf("explicit password should not print a generated password"))

	users, err := fs.BackofficeUserRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 1)
}

// TestBootstrap_IdempotentReRun pins the "ℹ️  already exists" branch:
// a second run with the same email is a no-op + exit 0.
func TestBootstrap_IdempotentReRun(t *testing.T) {
	c := qt.New(t)
	fs := setupMemoryAsPostgres(c)

	_, err := runBootstrap(c, "postgres://test",
		"--email=admin@example.com",
		"--name=Ops Admin",
	)
	c.Assert(err, qt.IsNil)

	out, err := runBootstrap(c, "postgres://test",
		"--email=admin@example.com",
		"--name=Ops Admin",
	)
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "already exists")

	users, err := fs.BackofficeUserRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 1)
}

// TestBootstrap_RefusesSecondWithoutForce pins the fresh-deployment
// safeguard: once any back-office user exists, a NEW email is rejected
// unless --force is passed.
func TestBootstrap_RefusesSecondWithoutForce(t *testing.T) {
	c := qt.New(t)
	fs := setupMemoryAsPostgres(c)

	_, err := runBootstrap(c, "postgres://test",
		"--email=first@example.com",
		"--name=First",
	)
	c.Assert(err, qt.IsNil)

	_, err = runBootstrap(c, "postgres://test",
		"--email=second@example.com",
		"--name=Second",
	)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "--force")

	users, err := fs.BackofficeUserRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 1)
}

// TestBootstrap_ForceAllowsSecond pins that --force unblocks the
// second-user path.
func TestBootstrap_ForceAllowsSecond(t *testing.T) {
	c := qt.New(t)
	fs := setupMemoryAsPostgres(c)

	_, err := runBootstrap(c, "postgres://test",
		"--email=first@example.com",
		"--name=First",
	)
	c.Assert(err, qt.IsNil)

	_, err = runBootstrap(c, "postgres://test",
		"--email=second@example.com",
		"--name=Second",
		"--force",
	)
	c.Assert(err, qt.IsNil)

	users, err := fs.BackofficeUserRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 2)
}

// TestBootstrap_RejectsMemoryDSN pins the persistence guard: a
// memory:// DSN is rejected by the shared DatabaseConfig.Validate at
// the database-config layer (cmd/inventario/shared/database.go: "only
// support PostgreSQL"), and the bootstrap CLI surfaces that wrapped
// error. The back-office plane intentionally has no memory backend
// because operator identities must survive process restarts.
func TestBootstrap_RejectsMemoryDSN(t *testing.T) {
	c := qt.New(t)

	_, err := runBootstrap(c, "memory://",
		"--email=admin@example.com",
		"--name=Ops Admin",
	)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "PostgreSQL")
}

// TestBootstrap_RejectsInvalidRole pins that the role flag is validated
// against the closed-set enum before the service is even constructed.
func TestBootstrap_RejectsInvalidRole(t *testing.T) {
	c := qt.New(t)
	setupMemoryAsPostgres(c)

	_, err := runBootstrap(c, "postgres://test",
		"--email=admin@example.com",
		"--name=Ops Admin",
		"--role=wizard",
	)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "--role")
}

// TestBootstrap_RequiresEmail pins the basic required-arg branch.
func TestBootstrap_RequiresEmail(t *testing.T) {
	c := qt.New(t)
	setupMemoryAsPostgres(c)

	_, err := runBootstrap(c, "postgres://test", "--name=Ops")
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "--email is required")
}

// TestBootstrap_RequiresName pins the basic required-arg branch.
func TestBootstrap_RequiresName(t *testing.T) {
	c := qt.New(t)
	setupMemoryAsPostgres(c)

	_, err := runBootstrap(c, "postgres://test", "--email=a@example.com")
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "--name is required")
}

// TestBootstrap_RejectsMalformedEmail pins that BackofficeUser model
// validation (EmailPattern via ValidateWithContext) runs inside the
// service layer — the registry's floor only checks "not empty + role
// enum", so without the model-level call a malformed email would
// round-trip into the table.
func TestBootstrap_RejectsMalformedEmail(t *testing.T) {
	c := qt.New(t)
	setupMemoryAsPostgres(c)

	_, err := runBootstrap(c, "postgres://test",
		"--email=not-an-email",
		"--name=Ops Admin",
	)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "validation failed")
}

// TestBootstrap_MFADefaultsEnforced pins the secure default: with no
// --mfa-enforced flag (and no config/env override) the seeded operator is
// created MFA-enforced. Guards against the regression PR #1994 review flagged —
// a plain bool / env-default could let an omitted value silently land as false.
func TestBootstrap_MFADefaultsEnforced(t *testing.T) {
	c := qt.New(t)
	fs := setupMemoryAsPostgres(c)

	_, err := runBootstrap(c, "postgres://test",
		"--email=admin@example.com",
		"--name=Ops Admin",
	)
	c.Assert(err, qt.IsNil)

	users, err := fs.BackofficeUserRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 1)
	c.Assert(users[0].MFAEnforced, qt.IsTrue,
		qt.Commentf("omitting --mfa-enforced must keep the secure default (MFA enforced)"))
}

// TestBootstrap_MFADisabledByFlag pins the demo/preview opt-out: an explicit
// --mfa-enforced=false provisions a password-only operator (MFAEnforced=false),
// so a Helm Job — which has no interactive terminal for TOTP enrolment — can
// seed an operator that can actually sign in at /backoffice/login (issue #1967).
func TestBootstrap_MFADisabledByFlag(t *testing.T) {
	c := qt.New(t)
	fs := setupMemoryAsPostgres(c)

	_, err := runBootstrap(c, "postgres://test",
		"--email=admin@example.com",
		"--name=Ops Admin",
		"--mfa-enforced=false",
	)
	c.Assert(err, qt.IsNil)

	users, err := fs.BackofficeUserRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 1)
	c.Assert(users[0].MFAEnforced, qt.IsFalse,
		qt.Commentf("--mfa-enforced=false must provision a password-only operator"))
}
