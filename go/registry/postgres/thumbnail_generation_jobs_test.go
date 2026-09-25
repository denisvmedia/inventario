package postgres_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
)

// A miss has to arrive as registry.ErrNotFound: the API layer's error mapping
// keys on the sentinel, so a raw driver error becomes a 500 for a question
// with a legitimate "no" answer.
func TestThumbnailGenerationJobRegistry_GetJobByFileID_NotFound(t *testing.T) {
	c := qt.New(t)

	_, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	db, err := sqlx.Open("postgres", skipIfNoPostgreSQL(t))
	c.Assert(err, qt.IsNil)
	defer db.Close()
	c.Assert(db.Ping(), qt.IsNil)

	// Service registry, because the thumbnail worker runs with no tenant in
	// context.
	jobRegistry := postgres.NewFactorySet(db).ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry()

	job, err := jobRegistry.GetJobByFileID(context.Background(), "non-existent-file-id")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(job, qt.IsNil)
}
