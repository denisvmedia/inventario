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
