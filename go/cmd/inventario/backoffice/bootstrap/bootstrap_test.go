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

// TestBootstrap_RejectsMemoryDSN pins the persistence guard: the
// memory backend has no place to keep rows across CLI invocations,
// so the command fails-closed with a hint.
func TestBootstrap_RejectsMemoryDSN(t *testing.T) {
	c := qt.New(t)

	// No memory-as-postgres alias here — we want the real memory:// DSN
	// path to flow through DatabaseConfig.Validate first. That validator
	// itself rejects non-postgres DSNs at the database config layer
	// (see cmd/inventario/shared/database.go: "only support PostgreSQL").
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
