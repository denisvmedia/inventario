package postgres_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
	"go.5x5.cz/inventario/schema/migrations/migrator"
	"go.5x5.cz/inventario/services"
)

// fileLinkPGDatabase is a database of its own with the full schema, a tenant,
// a user and a group, for a test that changes the schema under the registries.
type fileLinkPGDatabase struct {
	set *registry.Set
	dbx *sqlx.DB
	ctx context.Context
}

// newFileLinkPGDatabase builds the schema in a new database, so the test can
// alter it without touching the one the rest of the suite shares. The database
// is dropped when the test ends.
func newFileLinkPGDatabase(t *testing.T) fileLinkPGDatabase {
	t.Helper()
	c := qt.New(t)
	ctx := context.Background()

	baseDSN := skipIfNoPostgreSQL(t)
	admin, err := pgxpool.New(ctx, baseDSN)
	c.Assert(err, qt.IsNil)
	t.Cleanup(admin.Close)

	name := fmt.Sprintf("inventario_file_links_%d", os.Getpid())
	_, err = admin.Exec(ctx, `DROP DATABASE IF EXISTS `+name+` WITH (FORCE)`)
	c.Assert(err, qt.IsNil)
	_, err = admin.Exec(ctx, `CREATE DATABASE `+name)
	c.Assert(err, qt.IsNil)

	u, err := url.Parse(baseDSN)
	c.Assert(err, qt.IsNil)
	u.Path = "/" + name
	dsn := u.String()
	t.Cleanup(func() {
		closeSharedPool(dsn)
		_, _ = admin.Exec(context.Background(), `DROP DATABASE IF EXISTS `+name+` WITH (FORCE)`)
	})

	c.Assert(buildSchema(ctx, migrator.NewWithFallback(dsn, "../../models"), dsn), qt.IsNil)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)

	service := createRegistrySetFromPool(pool).CreateServiceRegistrySet()
	tenantID, userID := setupTestTenantAndUser(c, service)
	groupSlug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	group, err := service.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Name:                "Test Group",
		Slug:                groupSlug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           userID,
		GroupCurrency:       models.Currency("USD"),
	})
	c.Assert(err, qt.IsNil)

	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	set := postgres.NewRegistrySetWithUserAndGroupID(dbx, userID, tenantID, group.ID)
	return fileLinkPGDatabase{
		set: set,
		dbx: dbx,
		ctx: tagCtxFor(getTestUser(c, set), group.ID),
	}
}

// closeSharedPool closes the shared pool of dsn, so its database can be
// dropped without the pool reconnecting to it.
func closeSharedPool(dsn string) {
	poolMutex.Lock()
	defer poolMutex.Unlock()
	if pool, ok := sharedPools[dsn]; ok {
		pool.Close()
		delete(sharedPools, dsn)
	}
}

func legacyLinkTestFile(name string, links ...models.FileLink) models.FileEntity {
	return models.FileEntity{
		Title:    name,
		Type:     models.FileTypeDocument,
		Category: models.FileCategoryDocuments,
		Links:    links,
		File: &models.File{
			Path:         name,
			OriginalPath: "files/" + name,
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}
}

// The release that drops the legacy link columns of files migrates before its
// pods replace the ones of this release, which keep serving uploads and edits
// until the rollout ends (#2898). They must not name the columns.
func TestFileRegistry_Postgres_WorksWithoutLegacyLinkColumns(t *testing.T) {
	c := qt.New(t)
	db := newFileLinkPGDatabase(t)
	reg := db.set.FileRegistry

	before, err := reg.Create(db.ctx, legacyLinkTestFile("before", pgCommodityLink("tv", "documents")))
	c.Assert(err, qt.IsNil)

	_, err = db.dbx.ExecContext(db.ctx, `ALTER TABLE files
		DROP COLUMN linked_entity_type,
		DROP COLUMN linked_entity_id,
		DROP COLUMN linked_entity_meta`)
	c.Assert(err, qt.IsNil)

	created, err := reg.Create(db.ctx, legacyLinkTestFile("after", pgCommodityLink("tv", "images")))
	c.Assert(err, qt.IsNil)
	c.Assert(pgLinkKeys(created), qt.DeepEquals, []string{"commodity/tv/images"})

	for _, f := range []*models.FileEntity{before, created} {
		edit := *f
		edit.Title = f.Title + " edited"
		updated, err := reg.Update(db.ctx, edit)
		c.Assert(err, qt.IsNil)
		c.Assert(updated.Title, qt.Equals, f.Title+" edited")

		_, added, err := reg.AddLink(db.ctx, models.FileLink{
			FileID: f.ID, EntityType: models.FileLinkLocation, EntityID: "home", Meta: "files",
		})
		c.Assert(err, qt.IsNil)
		c.Assert(added, qt.IsTrue)

		got, err := reg.Get(db.ctx, f.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.LinkTo(models.FileLinkLocation, "home"), qt.IsNotNil)
	}
}

// A row an older release wrote keeps its legacy link through an edit, and the
// startup move still links it and empties the columns.
func TestMoveLegacyFileLinks_Postgres_EditedRow(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkPGFixture(t)
	reg := fx.set.FileRegistry

	file := fx.file(c, "manual", 1)
	_, err := fx.dbx.ExecContext(fx.ctx,
		`UPDATE files SET linked_entity_type = 'commodity', linked_entity_id = 'radio', linked_entity_meta = 'documents' WHERE id = $1`,
		file.ID)
	c.Assert(err, qt.IsNil)
	legacyColumns := func() string {
		var typ, id, meta string
		c.Assert(fx.dbx.QueryRowxContext(fx.ctx,
			`SELECT COALESCE(linked_entity_type, ''), COALESCE(linked_entity_id, ''), COALESCE(linked_entity_meta, '') FROM files WHERE id = $1`,
			file.ID).Scan(&typ, &id, &meta), qt.IsNil)
		return typ + "/" + id + "/" + meta
	}

	edit := *file
	edit.Title = "manual edited"
	_, err = reg.Update(fx.ctx, edit)
	c.Assert(err, qt.IsNil)
	c.Assert(legacyColumns(), qt.Equals, "commodity/radio/documents")

	moved, err := services.MoveLegacyFileLinks(fx.ctx, fx.factorySet)
	c.Assert(err, qt.IsNil)
	c.Assert(moved, qt.Equals, 1)

	got, err := reg.Get(fx.ctx, file.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(pgLinkKeys(got), qt.DeepEquals, []string{"commodity/radio/documents"})
	c.Assert(got.Title, qt.Equals, "manual edited")
	c.Assert(legacyColumns(), qt.Equals, "//")
}
