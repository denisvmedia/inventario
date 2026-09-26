package migrator_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	_ "github.com/jackc/pgx/v5/stdlib"

	"go.5x5.cz/inventario/internal/pgtest"
	"go.5x5.cz/inventario/schema/migrations/migrator"
)

// The startup schema check runs as the application, and the application
// connects as a role that bootstrap grants USAGE on public and nothing more.
// Reading the revision table the ordinary way goes through ptah's Initialize,
// which issues CREATE TABLE IF NOT EXISTS — and PostgreSQL checks the schema's
// CREATE privilege before the IF NOT EXISTS short-circuit, so it fails even
// when the table is right there. The app then refuses to start (#2577).
//
// This is DSN-gated because the property is a privilege check, and an
// in-memory registry has no privileges to deny.
func TestVerifySchemaUpToDate_DoesNotNeedCreate(t *testing.T) {
	c := qt.New(t)

	dsn := pgtest.DSN(t)

	ctx := context.Background()
	admin, err := sql.Open("pgx", dsn)
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() { _ = admin.Close() })

	// A login role with USAGE on public and no CREATE — the shape bootstrap
	// gives the application.
	const role = "inventario_schema_check_probe"
	const password = "probe-password"
	_, _ = admin.ExecContext(ctx, `DROP ROLE IF EXISTS `+role)
	_, err = admin.ExecContext(ctx, `CREATE ROLE `+role+` LOGIN PASSWORD '`+password+`'`)
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), `REASSIGN OWNED BY `+role+` TO CURRENT_USER`)
		_, _ = admin.ExecContext(context.Background(), `DROP OWNED BY `+role)
		_, _ = admin.ExecContext(context.Background(), `DROP ROLE IF EXISTS `+role)
	})

	_, err = admin.ExecContext(ctx, `GRANT USAGE ON SCHEMA public TO `+role)
	c.Assert(err, qt.IsNil)
	_, err = admin.ExecContext(ctx, `GRANT SELECT ON ALL TABLES IN SCHEMA public TO `+role)
	c.Assert(err, qt.IsNil)

	// The probe must not be able to create anything, or this proves nothing.
	probeDSN := swapCredentials(c, dsn, role, password)
	probe, err := sql.Open("pgx", probeDSN)
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() { _ = probe.Close() })
	_, err = probe.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_check_probe_guard (x int)`)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "permission denied",
		qt.Commentf("the probe role can create tables; the test would pass vacuously"))

	// The check itself: reads the version, creates nothing. Either answer is
	// fine — up to date, or behind — as long as it is not a permission error.
	err = migrator.NewWithFallback(probeDSN, "").VerifySchemaUpToDate(ctx)
	if err != nil {
		c.Assert(err.Error(), qt.Not(qt.Contains), "permission denied",
			qt.Commentf("the schema check still needs CREATE"))
	}
}

// swapCredentials rewrites the user and password of a postgres URL, keeping
// the host, database and query intact.
func swapCredentials(c *qt.C, dsn, user, password string) string {
	c.Helper()

	scheme, rest, ok := strings.Cut(dsn, "://")
	c.Assert(ok, qt.IsTrue, qt.Commentf("DSN is not a URL: %s", dsn))
	_, hostAndRest, ok := strings.Cut(rest, "@")
	c.Assert(ok, qt.IsTrue, qt.Commentf("DSN carries no credentials: %s", dsn))
	return scheme + "://" + user + ":" + password + "@" + hostAndRest
}

// The startup schema check also runs against a metadata table it does not own:
// the migration role creates schema_migrations, the application reads it, and
// keeping those two roles apart is the point of having them. Ptah 0.8 refuses
// to touch a metadata table the connection does not own — on reads too, because
// a foreign table can carry a policy or a default expression the planner
// evaluates — so the check cannot go through Ptah at all (#2707).
//
// The sibling test above creates the probe role but no table, and an absent
// table has no owner to conflict with. That is why every Go test passed on 0.8
// while the compose, kind and e2e stacks went red: the table has to exist AND
// belong to someone else before the refusal can fire.
func TestVerifySchemaUpToDate_ReadsATableItDoesNotOwn(t *testing.T) {
	c := qt.New(t)

	dsn := pgtest.DSN(t)

	ctx := context.Background()
	admin, err := sql.Open("pgx", dsn)
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() { _ = admin.Close() })

	// The table must exist for the ownership question to arise at all.
	var existed bool
	err = admin.QueryRowContext(ctx,
		`SELECT to_regclass('schema_migrations') IS NOT NULL`).Scan(&existed)
	c.Assert(err, qt.IsNil)
	if !existed {
		_, err = admin.ExecContext(ctx, `CREATE TABLE schema_migrations (
			version BIGINT PRIMARY KEY,
			description TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			state VARCHAR(32) NOT NULL DEFAULT 'applied'
		)`)
		c.Assert(err, qt.IsNil)
		t.Cleanup(func() {
			_, _ = admin.ExecContext(context.Background(), `DROP TABLE IF EXISTS schema_migrations`)
		})
	}

	const role = "inventario_owner_check_probe"
	const password = "probe-password"
	_, _ = admin.ExecContext(ctx, `DROP ROLE IF EXISTS `+role)
	_, err = admin.ExecContext(ctx, `CREATE ROLE `+role+` LOGIN PASSWORD '`+password+`'`)
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), `REASSIGN OWNED BY `+role+` TO CURRENT_USER`)
		_, _ = admin.ExecContext(context.Background(), `DROP OWNED BY `+role)
		_, _ = admin.ExecContext(context.Background(), `DROP ROLE IF EXISTS `+role)
	})
	_, err = admin.ExecContext(ctx, `GRANT USAGE ON SCHEMA public TO `+role)
	c.Assert(err, qt.IsNil)
	_, err = admin.ExecContext(ctx, `GRANT SELECT ON ALL TABLES IN SCHEMA public TO `+role)
	c.Assert(err, qt.IsNil)

	// Without this the test proves nothing: the refusal only fires when the
	// owner and the connected role differ.
	var owner string
	err = admin.QueryRowContext(ctx,
		`SELECT tableowner FROM pg_tables WHERE tablename = 'schema_migrations'`).Scan(&owner)
	c.Assert(err, qt.IsNil)
	c.Assert(owner, qt.Not(qt.Equals), role,
		qt.Commentf("the probe owns schema_migrations; the ownership refusal cannot fire"))

	probeDSN := swapCredentials(c, dsn, role, password)
	err = migrator.NewWithFallback(probeDSN, "").VerifySchemaUpToDate(ctx)
	if err != nil {
		c.Assert(err.Error(), qt.Not(qt.Contains), "owned by",
			qt.Commentf("the schema check still routes the read through Ptah's metadata path"))
		c.Assert(err.Error(), qt.Not(qt.Contains), "permission denied")
	}
}
