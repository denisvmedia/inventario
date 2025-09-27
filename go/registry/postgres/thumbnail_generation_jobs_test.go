//go:build integration

package postgres_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
)

func TestThumbnailGenerationJobRegistry_GetJobByFileID_NotFound(t *testing.T) {
	c := qt.New(t)

	// Connect to test database
	dsn := "postgres://inventario:inventario_password@localhost:5432/inventario?sslmode=disable"
	db, err := sqlx.Open("postgres", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	// Test connection
	err = db.Ping()
	c.Assert(err, qt.IsNil)

	// Create factory set with PostgreSQL
	factorySet := postgres.NewFactorySet(db)

	// Create service registry (bypasses RLS)
	jobRegistry := factorySet.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry()

	// Try to get a job for a non-existent file ID
	nonExistentFileID := "non-existent-file-id"
	job, err := jobRegistry.GetJobByFileID(context.Background(), nonExistentFileID)

	// Should return ErrNotFound, not a generic SQL error
	c.Assert(err, qt.Equals, registry.ErrNotFound)
	c.Assert(job, qt.IsNil)
}
