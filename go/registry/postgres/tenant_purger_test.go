package postgres_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/appctx"
	_ "go.5x5.cz/inventario/internal/fileblob" // register file:// driver
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
	"go.5x5.cz/inventario/registry/postgres/store"
)

// seededTenant bundles the ids a purge test needs to assert against one tenant.
type seededTenant struct {
	tenantID string
	userID   string
}

// seedTenantWithDependents creates a tenant + admin user + active group, then a
// representative spread of dependent rows across the FK graph: the inventory
// hierarchy (location/area/commodity), a file, a tag, and the two tenant-only
// auth tables (refresh_tokens, login_events). It returns the ids so the test
// can prove they vanish (or survive) after a purge.
func seedTenantWithDependents(c *qt.C, ctx context.Context, fs *registry.FactorySet, dbx *sqlx.DB, slug, email string) seededTenant {
	c.Helper()

	svc := fs.CreateServiceRegistrySet()

	tenant, err := fs.TenantRegistry.Create(ctx, models.Tenant{
		Name:   "Tenant " + slug,
		Slug:   slug,
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Email:               email,
		Name:                "Admin " + slug,
		IsActive:            true,
	}
	c.Assert(user.SetPassword("Password123"), qt.IsNil)
	createdUser, err := fs.UserRegistry.Create(ctx, user)
	c.Assert(err, qt.IsNil)

	groupSlug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	group, err := svc.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Name:                "Group " + slug,
		Slug:                groupSlug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           createdUser.ID,
		GroupCurrency:       models.Currency("USD"),
	})
	c.Assert(err, qt.IsNil)

	// User+group-aware set so the inventory/file/tag seeds land with the right
	// tenant + group + created_by stamped from context. The seed helpers read
	// the user + group (incl. currency) off the context for validation.
	userSet := postgres.NewRegistrySetWithUserAndGroupID(dbx, createdUser.ID, tenant.ID, group.ID)
	userCtx := appctx.WithUser(ctx, createdUser)
	userCtx = appctx.WithGroup(userCtx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: group.ID},
			TenantID: tenant.ID,
		},
		GroupCurrency: models.Currency("USD"),
	})
	areaID := seedTagArea(c, userSet, userCtx)
	seedTagCommodity(c, userSet, userCtx, areaID, "Widget "+slug)
	fileID := seedTagFile(c, userSet, userCtx, "file-"+slug)
	mustCreateTag(c, userSet.TagRegistry, userCtx, models.TagKindCommodity, "tag-"+slug)
	_, _, err = userSet.CommodityDraftRegistry.CreateOrGet(userCtx, models.CommodityDraft{ClientKey: "draft-" + slug})
	c.Assert(err, qt.IsNil)
	_, _, err = userSet.IntakeRegistry.CreateSession(userCtx,
		models.IntakeSession{ClientKey: "intake-" + slug, Origin: models.IntakeOriginUpload},
		[]registry.IntakeSourceInput{{FileIDs: []string{fileID}}}, 0)
	c.Assert(err, qt.IsNil)
	_, _, err = userSet.IntakeDocumentRegistry.CreateOrGet(userCtx, models.IntakeDocument{
		ExternalSystem: "paperless-ngx", ExternalDocumentID: slug, PayloadHash: "h",
	})
	c.Assert(err, qt.IsNil)

	// Tenant-only auth rows.
	_, err = svc.RefreshTokenRegistry.Create(ctx, models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: tenant.ID,
			UserID:   createdUser.ID,
		},
		TokenHash: "hash-" + slug,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	})
	c.Assert(err, qt.IsNil)

	uid := createdUser.ID
	_, err = svc.LoginEventRegistry.Create(ctx, models.LoginEvent{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		UserID:              &uid,
		Email:               email,
		Outcome:             models.LoginOutcomeOK,
		Method:              models.LoginMethodPassword,
	})
	c.Assert(err, qt.IsNil)

	// A weekly digest send record (#1586), whose user and tenant FKs are NO
	// ACTION.
	claimed, err := fs.WeeklyDigestSendRegistry.ClaimWeek(ctx, models.WeeklyDigestSend{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: tenant.ID, UserID: createdUser.ID},
		WeekStart:               time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)

	return seededTenant{tenantID: tenant.ID, userID: createdUser.ID}
}

// countRowsForTenant counts rows in a table for one tenant, reading under the
// background-worker role so RLS doesn't hide foreign-tenant rows (the same
// bypass the purger itself relies on).
func countRowsForTenant(c *qt.C, dbx *sqlx.DB, table, tenantID string) int {
	c.Helper()
	var n int
	err := store.DoAsBackgroundWorker(context.Background(), dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &n, "SELECT COUNT(*) FROM "+table+" WHERE tenant_id = $1", tenantID)
	})
	c.Assert(err, qt.IsNil)
	return n
}

// TestTenantPurger_Postgres_PurgesAllTenantDependents is the #2115 regression:
// a tenant carrying data across the FK graph (inventory hierarchy, file, tag,
// group, user, refresh_token, login_event) must be fully cleared by
// PurgeTenantDependents so the subsequent tenants DELETE succeeds. A second
// tenant's rows must survive untouched, and a repeat purge must be a no-op.
func TestTenantPurger_Postgres_PurgesAllTenantDependents(t *testing.T) {
	fs := setupCleanPostgresFactorySet(t)

	c := qt.New(t)
	ctx := context.Background()

	dsn := skipIfNoPostgreSQL(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")

	a := seedTenantWithDependents(c, ctx, fs, dbx, "tenant-a", "admin@a.example")
	b := seedTenantWithDependents(c, ctx, fs, dbx, "tenant-b", "admin@b.example")

	purger := postgres.NewTenantPurger(dbx)

	err = purger.PurgeTenantDependents(ctx, a.tenantID)
	c.Assert(err, qt.IsNil)

	// Every dependent table is empty for tenant A.
	for _, table := range []string{
		"locations", "areas", "commodities", "files", "tags", "commodity_drafts",
		"intake_sessions", "intake_sources", "intake_documents", "location_groups", "users",
		"refresh_tokens", "login_events", "weekly_digest_sends",
	} {
		c.Assert(countRowsForTenant(c, dbx, table, a.tenantID), qt.Equals, 0,
			qt.Commentf("table %s should be empty for purged tenant A", table))
	}

	// The final tenants DELETE is what was broken before the fix: any orphaned
	// dependent would trip a NO ACTION FK here. It succeeding is the regression.
	err = fs.TenantRegistry.Delete(ctx, a.tenantID)
	c.Assert(err, qt.IsNil)
	_, err = fs.TenantRegistry.Get(ctx, a.tenantID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Tenant B is untouched — same table set still carries its rows.
	for _, table := range []string{
		"locations", "areas", "commodities", "files", "tags", "commodity_drafts",
		"intake_sessions", "intake_sources", "intake_documents", "location_groups", "users",
		"refresh_tokens", "login_events", "weekly_digest_sends",
	} {
		c.Assert(countRowsForTenant(c, dbx, table, b.tenantID) > 0, qt.IsTrue,
			qt.Commentf("table %s must still carry tenant B rows", table))
	}

	// Idempotent: a second purge after the rows are gone is a clean no-op.
	err = purger.PurgeTenantDependents(ctx, a.tenantID)
	c.Assert(err, qt.IsNil)
}

// The purge leaves the operators' part of a tenant's audit trail: every
// back-office action and every row written inside an impersonation session.
func TestTenantPurger_Postgres_KeepsTheOperatorsAuditTrail(t *testing.T) {
	fs := setupCleanPostgresFactorySet(t)

	c := qt.New(t)
	ctx := context.Background()

	dsn := skipIfNoPostgreSQL(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")

	a := seedTenantWithDependents(c, ctx, fs, dbx, "tenant-audit", "admin@audit.example")
	operator := "operator-1"
	for _, entry := range []models.AuditLog{
		{Action: "login", UserID: &a.userID},
		{Action: "admin.tenant_suspend", UserID: &operator},
		{Action: "commodity.update", UserID: &a.userID, ImpersonatedBy: &operator},
	} {
		entry.TenantID = &a.tenantID
		entry.Success = true
		_, err := fs.AuditLogRegistry.Create(ctx, entry)
		c.Assert(err, qt.IsNil)
	}

	c.Assert(postgres.NewTenantPurger(dbx).PurgeTenantDependents(ctx, a.tenantID), qt.IsNil)

	var actions []string
	err = store.DoAsBackgroundWorker(ctx, dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &actions, "SELECT action FROM audit_logs WHERE tenant_id = $1 ORDER BY action", a.tenantID)
	})
	c.Assert(err, qt.IsNil)
	c.Assert(actions, qt.DeepEquals, []string{"admin.tenant_suspend", "commodity.update"})
}

// CheckTenantPurge runs the purge and the tenants DELETE without keeping
// either, and reports a foreign key that would refuse them before the caller
// deletes anything.
func TestTenantPurger_Postgres_CheckTenantPurge(t *testing.T) {
	fs := setupCleanPostgresFactorySet(t)

	c := qt.New(t)
	ctx := context.Background()

	dsn := skipIfNoPostgreSQL(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	purger := postgres.NewTenantPurger(dbx)

	a := seedTenantWithDependents(c, ctx, fs, dbx, "tenant-check", "admin@check.example")

	c.Assert(purger.CheckTenantPurge(ctx, a.tenantID), qt.IsNil)
	_, err = fs.TenantRegistry.Get(ctx, a.tenantID)
	c.Assert(err, qt.IsNil)
	c.Assert(countRowsForTenant(c, dbx, "users", a.tenantID), qt.Equals, 1)
	c.Assert(countRowsForTenant(c, dbx, "weekly_digest_sends", a.tenantID), qt.Equals, 1)

	// A table the purge does not know, whose foreign key to users refuses
	// the purge.
	_, err = dbx.ExecContext(ctx, `CREATE TABLE purge_check_blocker (user_id TEXT NOT NULL REFERENCES users(id))`)
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() { _, _ = dbx.ExecContext(context.Background(), `DROP TABLE IF EXISTS purge_check_blocker`) })
	_, err = dbx.ExecContext(ctx, `INSERT INTO purge_check_blocker (user_id) VALUES ($1)`, a.userID)
	c.Assert(err, qt.IsNil)

	c.Assert(purger.CheckTenantPurge(ctx, a.tenantID), qt.IsNotNil)
	c.Assert(countRowsForTenant(c, dbx, "users", a.tenantID), qt.Equals, 1)
	c.Assert(countRowsForTenant(c, dbx, "files", a.tenantID), qt.Equals, 1)
}

// tenantPurgeExclusions are the tables with a tenant_id column that the
// tenant purge leaves alone, with the reason.
var tenantPurgeExclusions = map[string]string{}

// Every table that holds a tenant's rows is purged, and every table whose
// foreign key to tenants or users would refuse the deletion of the parent is
// among them. A table added without a place in the purge order fails here
// instead of failing a tenant deletion after the tenant's files are gone.
func TestTenantPurger_Postgres_CoversEveryTenantAndUserReference(t *testing.T) {
	c := qt.New(t)

	dsn := skipIfNoPostgreSQL(t)
	_, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	db, err := sql.Open("pgx", dsn)
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() { _ = db.Close() })

	purged := make(map[string]bool)
	for _, table := range postgres.TenantPurgeTableNames() {
		purged[table] = true
	}

	query := func(q string) []string {
		rows, err := db.QueryContext(context.Background(), q)
		c.Assert(err, qt.IsNil)
		defer rows.Close()
		var out []string
		for rows.Next() {
			var v string
			c.Assert(rows.Scan(&v), qt.IsNil)
			out = append(out, v)
		}
		c.Assert(rows.Err(), qt.IsNil)
		return out
	}

	// Foreign keys whose ON DELETE is NO ACTION or RESTRICT: the parent row
	// cannot go while a child row exists.
	blocking := query(`
		SELECT DISTINCT c.conrelid::regclass::text
		FROM pg_constraint c
		WHERE c.contype = 'f'
		  AND c.confrelid IN ('tenants'::regclass, 'users'::regclass)
		  AND c.confdeltype IN ('a', 'r')
		  AND c.conrelid NOT IN ('tenants'::regclass)
		ORDER BY 1`)
	c.Assert(len(blocking) > 20, qt.IsTrue, qt.Commentf("expected foreign keys to tenants and users, found %v", blocking))
	var missing []string
	for _, table := range blocking {
		if !purged[table] {
			missing = append(missing, table)
		}
	}
	c.Check(missing, qt.HasLen, 0,
		qt.Commentf("these tables reference tenants or users with NO ACTION but the tenant purge does not delete from them: %v", missing))

	// Every table with a tenant_id column, so rows the purge leaves behind
	// do not outlive their tenant.
	scoped := query(`
		SELECT table_name
		FROM information_schema.columns
		WHERE table_schema = current_schema() AND column_name = 'tenant_id'
		ORDER BY 1`)
	present := make(map[string]bool, len(scoped))
	missing = nil
	for _, table := range scoped {
		present[table] = true
		if !purged[table] {
			if _, ok := tenantPurgeExclusions[table]; !ok {
				missing = append(missing, table)
			}
		}
	}
	c.Check(missing, qt.HasLen, 0, qt.Commentf("these tables have a tenant_id but the tenant purge does not delete from them: %v", missing))

	for table := range purged {
		c.Check(present[table], qt.IsTrue, qt.Commentf("%s is in the purge order but has no tenant_id column", table))
	}
	for table, reason := range tenantPurgeExclusions {
		c.Check(present[table], qt.IsTrue, qt.Commentf("%s is excluded (%s) but has no tenant_id column", table, reason))
	}
}
