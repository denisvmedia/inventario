package postgres_test

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// #2633: the audit_logs tenant policy is a backstop, and a backstop nobody
// exercises is an annotation. This drives it from the database side.
//
// It needs a role that is NOT the table owner and NOT a member of
// inventario_background_worker: PostgreSQL does not apply a table's policies to
// its owner, and the worker policy is USING (true), so either one turns the
// isolation this asserts into nothing. The login role every other test uses is
// both, which is why this one builds its own.
func TestAuditLogs_TenantPolicyHidesEveryOtherTenant(t *testing.T) {
	c := qt.New(t)

	dsn := skipIfNoPostgreSQL(t)
	_, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	ctx := context.Background()
	owner, err := sql.Open("pgx", dsn)
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() { _ = owner.Close() })

	// A prefix unique to this test, so the assertions describe its own rows and
	// not whatever else the suite has audited.
	const action = "rls_probe.audit_isolation"
	rows := []struct {
		id     string
		tenant *string
	}{
		{id: action + ".a", tenant: new("tenant-a")},
		{id: action + ".b", tenant: new("tenant-b")},
		{id: action + ".none", tenant: nil},
	}
	for _, row := range rows {
		_, err = owner.ExecContext(ctx,
			`INSERT INTO audit_logs (id, tenant_id, action) VALUES ($1, $2, $3)`,
			row.id, row.tenant, action)
		c.Assert(err, qt.IsNil)
	}
	t.Cleanup(func() {
		_, _ = owner.ExecContext(context.Background(), `DELETE FROM audit_logs WHERE action = $1`, action)
	})

	const role = "inventario_audit_rls_probe"
	const password = "probe-password"
	_, _ = owner.ExecContext(ctx, `DROP ROLE IF EXISTS `+role)
	_, err = owner.ExecContext(ctx, `CREATE ROLE `+role+` LOGIN PASSWORD '`+password+`'`)
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() {
		_, _ = owner.ExecContext(context.Background(), `REASSIGN OWNED BY `+role+` TO CURRENT_USER`)
		_, _ = owner.ExecContext(context.Background(), `DROP OWNED BY `+role)
		_, _ = owner.ExecContext(context.Background(), `DROP ROLE IF EXISTS `+role)
	})
	_, err = owner.ExecContext(ctx, `GRANT USAGE ON SCHEMA public TO `+role)
	c.Assert(err, qt.IsNil)
	// The membership the policy is addressed to, and nothing else.
	_, err = owner.ExecContext(ctx, `GRANT inventario_app TO `+role)
	c.Assert(err, qt.IsNil)

	// Both of these would make every assertion below pass for the wrong reason.
	var isOwner, isWorker bool
	err = owner.QueryRowContext(ctx,
		`SELECT tableowner = $1 FROM pg_tables WHERE tablename = 'audit_logs'`, role).Scan(&isOwner)
	c.Assert(err, qt.IsNil)
	c.Assert(isOwner, qt.IsFalse, qt.Commentf("the probe owns audit_logs; policies do not apply to an owner"))
	err = owner.QueryRowContext(ctx,
		`SELECT pg_has_role($1, 'inventario_background_worker', 'MEMBER')`, role).Scan(&isWorker)
	c.Assert(err, qt.IsNil)
	c.Assert(isWorker, qt.IsFalse, qt.Commentf("the probe inherits the worker role's USING (true) policy"))

	probe, err := sql.Open("pgx", swapDSNCredentials(c, dsn, role, password))
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() { _ = probe.Close() })

	// visible reports the ids the probe can read with the tenant context set to
	// tenant, or with none set when tenant is empty. The context is
	// transaction-local, so the read has to happen inside the transaction that
	// set it.
	visible := func(tenant string) []string {
		tx, err := probe.BeginTx(ctx, nil)
		c.Assert(err, qt.IsNil)
		defer func() { _ = tx.Rollback() }()

		if tenant != "" {
			_, err = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id', $1, true)`, tenant)
			c.Assert(err, qt.IsNil)
		}
		found, err := tx.QueryContext(ctx, `SELECT id FROM audit_logs WHERE action = $1`, action)
		c.Assert(err, qt.IsNil, qt.Commentf("the probe cannot read audit_logs at all"))
		defer found.Close()

		var ids []string
		for found.Next() {
			var id string
			c.Assert(found.Scan(&id), qt.IsNil)
			ids = append(ids, id)
		}
		c.Assert(found.Err(), qt.IsNil)
		sort.Strings(ids)
		return ids
	}

	c.Run("a tenant sees its own row and no other", func(c *qt.C) {
		c.Assert(visible("tenant-a"), qt.DeepEquals, []string{action + ".a"})
		c.Assert(visible("tenant-b"), qt.DeepEquals, []string{action + ".b"})
	})

	// The nullable column is the part worth pinning down: a NULL tenant_id
	// fails the predicate rather than matching everyone, so a system event
	// belongs to no tenant instead of to all of them.
	c.Run("a row with no tenant belongs to no tenant", func(c *qt.C) {
		for _, tenant := range []string{"tenant-a", "tenant-b"} {
			c.Assert(visible(tenant), qt.Not(qt.Contains), action+".none")
		}
	})

	c.Run("no tenant context reads nothing", func(c *qt.C) {
		c.Assert(visible(""), qt.HasLen, 0)
	})
}

// swapDSNCredentials rewrites the user and password of a postgres URL, keeping
// the host, database and query intact.
func swapDSNCredentials(c *qt.C, dsn, user, password string) string {
	c.Helper()

	scheme, rest, ok := strings.Cut(dsn, "://")
	c.Assert(ok, qt.IsTrue, qt.Commentf("DSN is not a URL: %s", dsn))
	at := strings.LastIndex(rest, "@")
	c.Assert(at >= 0, qt.IsTrue, qt.Commentf("DSN carries no credentials: %s", dsn))
	return scheme + "://" + user + ":" + password + "@" + rest[at+1:]
}
