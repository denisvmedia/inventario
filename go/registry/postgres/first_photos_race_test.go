package postgres_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/postgres"
)

// afterQuery runs run once, right after the first query whose SQL contains
// match has finished, before the caller's next statement.
type afterQuery struct {
	match string
	once  sync.Once
	run   func()
}

type afterQueryKey struct{}

func (a *afterQuery) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, a.match) {
		return context.WithValue(ctx, afterQueryKey{}, true)
	}
	return ctx
}

func (a *afterQuery) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if ctx.Value(afterQueryKey{}) == true {
		a.once.Do(a.run)
	}
}

// FirstPhotos picks each item's photo in one statement and reads the picked
// files in a second. A photo moved to Trash in between is no cover (#2900).
func TestFirstPhotos_Postgres_LeavesOutAPhotoTrashedAfterThePick(t *testing.T) {
	c := qt.New(t)
	fx := newTrashPGFixture(t)

	lamp := fx.item(c, "Lamp")
	photo, err := fx.set.FileRegistry.Create(fx.ctx, models.FileEntity{
		Title:    "lamp.jpg",
		Type:     models.FileTypeImage,
		Category: models.FileCategoryImages,
		Links:    []models.FileLink{pgCommodityLink(lamp.ID, models.FileLinkMetaImages)},
		File: &models.File{
			Path:         "lamp",
			OriginalPath: "files/lamp.jpg",
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	})
	c.Assert(err, qt.IsNil)

	// The read under test runs on a pool of its own, which trashes the photo
	// through the fixture's pool as soon as the pick has finished.
	trashed := false
	tracer := &afterQuery{match: "DISTINCT ON (l.entity_id)", run: func() {
		c.Check(fx.set.FileRegistry.Trash(fx.ctx, photo.ID, fx.user.ID, "", time.Now()), qt.IsNil)
		trashed = true
	}}
	config, err := pgxpool.ParseConfig(skipIfNoPostgreSQL(t))
	c.Assert(err, qt.IsNil)
	config.ConnConfig.Tracer = tracer
	config.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	c.Assert(err, qt.IsNil)
	t.Cleanup(pool.Close)
	traced := postgres.NewFactorySet(sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx"))
	set, err := traced.CreateUserRegistrySet(fx.ctx)
	c.Assert(err, qt.IsNil)

	first, err := set.FileRegistry.FirstPhotos(fx.ctx, []string{lamp.ID})
	c.Assert(err, qt.IsNil)
	c.Assert(trashed, qt.IsTrue, qt.Commentf("the photo must go to Trash between the two statements"))
	c.Assert(first, qt.HasLen, 0)
}
