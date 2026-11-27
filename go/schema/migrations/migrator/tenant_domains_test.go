package migrator_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	_ "github.com/jackc/pgx/v5/stdlib"

	"go.5x5.cz/inventario/internal/pgtest"
	"go.5x5.cz/inventario/schema/migrations/migrator"
)

// legacyTenantsDatabase creates a database that holds a tenants table of the
// shape the rows written before #2948 had, with no unique index on domain,
// and returns its DSN and a connection to it. The database is dropped when
// the test ends.
func legacyTenantsDatabase(c *qt.C) (dsn string, db *sql.DB) {
	c.Helper()
	ctx := context.Background()

	baseDSN := pgtest.DSN(c.TB)
	admin, err := sql.Open("pgx", baseDSN)
	c.Assert(err, qt.IsNil)
	c.Cleanup(func() { _ = admin.Close() })

	name := fmt.Sprintf("inventario_tenant_domains_%d", os.Getpid())
	_, err = admin.ExecContext(ctx, `DROP DATABASE IF EXISTS `+name+` WITH (FORCE)`)
	c.Assert(err, qt.IsNil)
	_, err = admin.ExecContext(ctx, `CREATE DATABASE `+name)
	c.Assert(err, qt.IsNil)
	c.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), `DROP DATABASE IF EXISTS `+name+` WITH (FORCE)`)
	})

	u, err := url.Parse(baseDSN)
	c.Assert(err, qt.IsNil)
	u.Path = "/" + name
	dsn = u.String()

	db, err = sql.Open("pgx", dsn)
	c.Assert(err, qt.IsNil)
	c.Cleanup(func() { _ = db.Close() })

	// MigrateUp checks the extensions before the tenant domains.
	_, err = db.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS pg_trgm`)
	c.Assert(err, qt.IsNil)
	_, err = db.ExecContext(ctx, `CREATE TABLE tenants (id TEXT PRIMARY KEY, slug TEXT NOT NULL, domain TEXT)`)
	c.Assert(err, qt.IsNil)
	_, err = db.ExecContext(ctx, `INSERT INTO tenants (id, slug, domain) VALUES
		('1', 'acme', 'acme.example'),
		('2', 'acme-old', 'acme.example'),
		('3', 'globex-eu', 'globex.example'),
		('4', 'globex', 'globex.example'),
		('5', 'initech', 'initech.example'),
		('6', 'umbrella', NULL),
		('7', 'hooli', NULL)`)
	c.Assert(err, qt.IsNil)

	return dsn, db
}

// A database whose tenants share a domain cannot take the unique index on
// tenants.domain. MigrateUp says which domains and tenants before it applies
// anything, instead of failing on the index halfway through the chain.
func TestMigrateUp_StopsWhenTenantsShareADomain(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	dsn, db := legacyTenantsDatabase(c)

	err := migrator.NewWithFallback(dsn, "").MigrateUp(ctx, migrator.Args{})
	c.Assert(err, qt.ErrorIs, migrator.ErrDuplicateTenantDomains)
	c.Assert(err, qt.ErrorMatches,
		`tenants share a domain, so tenants.domain cannot be made unique: `+
			`"acme.example" \(tenants acme, acme-old\); "globex.example" \(tenants globex, globex-eu\)\. .*`+
			"`inventario tenants update <slug> --domain=<domain>`.*")

	var migrated bool
	err = db.QueryRowContext(ctx, `SELECT to_regclass('schema_migrations') IS NOT NULL`).Scan(&migrated)
	c.Assert(err, qt.IsNil)
	c.Assert(migrated, qt.IsFalse, qt.Commentf("MigrateUp started migrating a database it should have refused"))
}

// A dry run reports the shared domains too, so an operator can check a
// database before the deploy that migrates it.
func TestMigrateUp_DryRunStopsWhenTenantsShareADomain(t *testing.T) {
	c := qt.New(t)
	dsn, _ := legacyTenantsDatabase(c)

	err := migrator.NewWithFallback(dsn, "").MigrateUp(context.Background(), migrator.Args{DryRun: true})
	c.Assert(err, qt.ErrorIs, migrator.ErrDuplicateTenantDomains)
}

// Once every domain is left with one tenant, the check lets the migrations
// run. Tenants without a domain never count as sharing one.
func TestMigrateUp_LetsTenantsWithOneDomainEachThrough(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	dsn, db := legacyTenantsDatabase(c)
	_, err := db.ExecContext(ctx, `UPDATE tenants SET domain = NULL WHERE slug IN ('acme-old', 'globex-eu')`)
	c.Assert(err, qt.IsNil)

	err = migrator.NewWithFallback(dsn, "").MigrateUp(ctx, migrator.Args{DryRun: true})
	c.Assert(err, qt.IsNil)
}
