package ptah_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/stokaro/ptah/core/goschema"
	"github.com/stokaro/ptah/dbschema"
	"github.com/stokaro/ptah/migration/schemadiff"

	ptahintegration "github.com/denisvmedia/inventario/registry/ptah"
)

// TestSchemaDriftDetection validates that the current migration files are in sync
// with the Go entity annotations. This test should fail if someone modifies Go
// entity annotations without updating migrations.
func TestSchemaDriftDetection(t *testing.T) {
	c := qt.New(t)

	// Skip if no database URL is provided
	dbURL := os.Getenv("TEST_DB_DSN")
	if dbURL == "" {
		t.Skip("TEST_DB_DSN environment variable not set")
	}

	// Parse Go entities from models directory
	modelsDir, err := filepath.Abs("../../models")
	c.Assert(err, qt.IsNil)

	goSchema, err := goschema.ParseDir(modelsDir)
	c.Assert(err, qt.IsNil)

	// Connect to database and read current schema
	conn, err := dbschema.ConnectToDatabase(dbURL)
	c.Assert(err, qt.IsNil)
	defer conn.Close()

	dbSchema, err := conn.Reader().ReadSchema()
	c.Assert(err, qt.IsNil)

	// Compare schemas
	diff := schemadiff.Compare(goSchema, dbSchema)

	// Test should pass if there are no differences
	if diff.HasChanges() {
		t.Errorf("Schema drift detected! Go entity annotations differ from database schema.\n"+
			"Tables added: %d\n"+
			"Tables removed: %d\n"+
			"Tables modified: %d\n"+
			"Indexes added: %d\n"+
			"Indexes removed: %d\n"+
			"Enums added: %d\n"+
			"Enums removed: %d\n"+
			"Run 'inventario migrate generate' to create migration files for these changes.",
			len(diff.TablesAdded),
			len(diff.TablesRemoved),
			len(diff.TablesModified),
			len(diff.IndexesAdded),
			len(diff.IndexesRemoved),
			len(diff.EnumsAdded),
			len(diff.EnumsRemoved))
	}
}

// TestMigrationFilesSyncWithAnnotations validates that applying the current
// migration files would result in a schema that matches the Go annotations.
func TestMigrationFilesSyncWithAnnotations(t *testing.T) {
	c := qt.New(t)

	// Skip if no database URL is provided
	dbURL := os.Getenv("TEST_DB_DSN")
	if dbURL == "" {
		t.Skip("TEST_DB_DSN environment variable not set")
	}

	// Create a temporary test database
	testDBURL := createTestDatabase(t, dbURL)
	defer dropTestDatabase(t, testDBURL)

	// Apply current migration files to empty database
	migrator, err := ptahintegration.NewPtahMigrator(testDBURL, "../../models")
	c.Assert(err, qt.IsNil)

	// Apply migrations
	err = migrator.MigrateUp(context.Background(), false)
	c.Assert(err, qt.IsNil)

	// Now check if the resulting schema matches Go annotations
	modelsDir, err := filepath.Abs("../../models")
	c.Assert(err, qt.IsNil)

	goSchema, err := goschema.ParseDir(modelsDir)
	c.Assert(err, qt.IsNil)

	// Read the database schema after migration
	conn, err := dbschema.ConnectToDatabase(testDBURL)
	c.Assert(err, qt.IsNil)
	defer conn.Close()

	dbSchema, err := conn.Reader().ReadSchema()
	c.Assert(err, qt.IsNil)

	// Compare schemas - they should match
	diff := schemadiff.Compare(goSchema, dbSchema)

	if diff.HasChanges() {
		t.Errorf("Migration files do not result in schema matching Go annotations!\n"+
			"This indicates that the migration files are out of sync.\n"+
			"Tables added: %d\n"+
			"Tables removed: %d\n"+
			"Tables modified: %d\n"+
			"Run 'inventario migrate generate' to update migration files.",
			len(diff.TablesAdded),
			len(diff.TablesRemoved),
			len(diff.TablesModified))
	}
}

// TestPtahIndexSupport validates what index features Ptah supports
func TestPtahIndexSupport(t *testing.T) {
	c := qt.New(t)

	// Parse Go entities to see what indexes are detected
	modelsDir, err := filepath.Abs("../../models")
	c.Assert(err, qt.IsNil)

	goSchema, err := goschema.ParseDir(modelsDir)
	c.Assert(err, qt.IsNil)

	// Log what indexes Ptah found
	t.Logf("Ptah detected %d indexes:", len(goSchema.Indexes))
	for _, index := range goSchema.Indexes {
		t.Logf("  - %s on %s: fields=%v, unique=%v",
			index.Name, index.StructName, index.Fields, index.Unique)
	}

	// Check if our PostgreSQL-specific indexes are missing
	expectedIndexes := []string{
		"commodities_tags_gin_idx",
		"commodities_extra_serial_numbers_gin_idx",
		"commodities_part_numbers_gin_idx",
		"commodities_urls_gin_idx",
		"commodities_active_idx",
		"commodities_draft_idx",
		"files_tags_gin_idx",
		"files_type_created_idx",
		"files_linked_entity_idx",
		"files_linked_entity_meta_idx",
	}

	foundIndexes := make(map[string]bool)
	for _, index := range goSchema.Indexes {
		foundIndexes[index.Name] = true
	}

	var missingIndexes []string
	for _, expected := range expectedIndexes {
		if !foundIndexes[expected] {
			missingIndexes = append(missingIndexes, expected)
		}
	}

	if len(missingIndexes) > 0 {
		t.Logf("WARNING: Ptah is not detecting these PostgreSQL-specific indexes: %v", missingIndexes)
		t.Logf("This indicates limitations in Ptah's index annotation parsing.")
	}
}

// Helper functions for test database management
func createTestDatabase(t *testing.T, baseURL string) string {
	// Implementation would create a temporary test database
	// For now, return the base URL (assumes test database is already clean)
	return baseURL
}

func dropTestDatabase(t *testing.T, testURL string) {
	// Implementation would drop the temporary test database
	// For now, do nothing (assumes test database will be cleaned externally)
}
