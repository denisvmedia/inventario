package postgres_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/postgres"
)

// TestFileRegistry_Postgres_ListByLinkedEntity_AreaAndLocation exercises the
// ListByLinkedEntity SQL path — the query the #2119 fix
// (EntityService → FileService.DeleteLinkedFiles) depends on — for the 'area'
// and 'location' link types against a real PostgreSQL schema. The query
// itself carries no tenant filter (RLS supplies it), so the cross-group
// isolation subtest is the load-bearing assertion: a user from another
// tenant/group must see nothing for the same linked-entity id.
func TestFileRegistry_Postgres_ListByLinkedEntity_AreaAndLocation(t *testing.T) {
	c := qt.New(t)

	set, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	user := getTestUser(c, set)
	ctx := appctx.WithUser(c.Context(), user)

	mk := func(name, linkedType, linkedID string) models.FileEntity {
		return models.FileEntity{
			Title:            name,
			Type:             models.FileTypeDocument,
			Category:         models.FileCategoryDocuments,
			LinkedEntityType: linkedType,
			LinkedEntityID:   linkedID,
			LinkedEntityMeta: "images",
			File: &models.File{
				Path:         name,
				OriginalPath: name + ".pdf",
				Ext:          ".pdf",
				MIMEType:     "application/pdf",
			},
		}
	}

	// The linked-entity link is polymorphic (no FK), so plain ids suffice —
	// no real area/location rows are required for the query under test.
	seed := []models.FileEntity{
		mk("area-doc-1", "area", "area-1"),
		mk("area-doc-2", "area", "area-1"),
		mk("loc-doc-1", "location", "loc-1"),
		mk("com-doc-1", "commodity", "com-1"),
	}
	for _, fe := range seed {
		fe.TenantID = user.TenantID
		fe.CreatedByUserID = user.ID
		_, err := set.FileRegistry.Create(ctx, fe)
		c.Assert(err, qt.IsNil)
	}

	t.Run("lists files linked to an area", func(t *testing.T) {
		c := qt.New(t)
		got, err := set.FileRegistry.ListByLinkedEntity(ctx, "area", "area-1")
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.HasLen, 2)
		for _, f := range got {
			c.Assert(f.LinkedEntityType, qt.Equals, "area")
			c.Assert(f.LinkedEntityID, qt.Equals, "area-1")
		}
	})

	t.Run("lists files linked to a location", func(t *testing.T) {
		c := qt.New(t)
		got, err := set.FileRegistry.ListByLinkedEntity(ctx, "location", "loc-1")
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.HasLen, 1)
		c.Assert(got[0].LinkedEntityType, qt.Equals, "location")
		c.Assert(got[0].LinkedEntityID, qt.Equals, "loc-1")
	})

	t.Run("empty slice for an entity with no files", func(t *testing.T) {
		c := qt.New(t)
		got, err := set.FileRegistry.ListByLinkedEntity(ctx, "area", "no-such-area")
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.HasLen, 0)
	})

	t.Run("type mismatch yields nothing", func(t *testing.T) {
		c := qt.New(t)
		// Same id, wrong link type — the WHERE clause is on the pair.
		got, err := set.FileRegistry.ListByLinkedEntity(ctx, "location", "area-1")
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.HasLen, 0)
	})

	t.Run("RLS isolates another tenant and group", func(t *testing.T) {
		c := qt.New(t)

		dsn := skipIfNoPostgreSQL(t)
		pool, err := getOrCreatePool(dsn)
		c.Assert(err, qt.IsNil)
		dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
		fs := postgres.NewFactorySet(dbx)

		bgCtx := context.Background()
		tenantB := mustCreateTenant(c, bgCtx, fs, "tenant-b-files")
		userB := mustCreateUser(c, bgCtx, fs, tenantB, "user-b@files.example")
		groupB := mustCreateActiveGroup(c, bgCtx, fs, tenantB, userB.ID)

		setB := postgres.NewRegistrySetWithUserAndGroupID(dbx, userB.ID, tenantB, groupB)
		ctxB := appctx.WithUser(bgCtx, userB)

		// The same (type, id) pair that has two rows in group A yields NOTHING
		// for a user scoped to another tenant/group.
		got, err := setB.FileRegistry.ListByLinkedEntity(ctxB, "area", "area-1")
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.HasLen, 0,
			qt.Commentf("RLS must hide group A's area-linked files from tenant/group B"))
	})
}
