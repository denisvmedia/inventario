package commonsql

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/denisvmedia/inventario/models"
)

func TestExportRegistry_Create(t *testing.T) {
	db := PrepareTestDB()

	registry := NewExportRegistry(db)

	export := models.Export{
		Type:            models.ExportTypeFullDatabase,
		Status:          models.ExportStatusPending,
		IncludeFileData: true,
		Description:     "Test export",
		CreatedDate:     models.PDate{},
	}

	result, err := registry.Create(context.Background(), export)
	require.NoError(t, err)
	assert.NotEmpty(t, result.GetID())
	assert.Equal(t, models.ExportTypeFullDatabase, result.Type)
	assert.Equal(t, models.ExportStatusPending, result.Status)
	assert.True(t, result.IncludeFileData)
	assert.Equal(t, "Test export", result.Description)
}

func TestExportRegistry_Get(t *testing.T) {
	db := PrepareTestDB()

	registry := NewExportRegistry(db)

	// Create an export first
	export := models.Export{
		Type:            models.ExportTypeCommodities,
		Status:          models.ExportStatusPending,
		IncludeFileData: false,
		Description:     "Test get export",
		CreatedDate:     models.PDate{},
	}

	created, err := registry.Create(context.Background(), export)
	require.NoError(t, err)

	// Get the export
	result, err := registry.Get(context.Background(), created.GetID())
	require.NoError(t, err)
	assert.Equal(t, created.GetID(), result.GetID())
	assert.Equal(t, models.ExportTypeCommodities, result.Type)
	assert.Equal(t, models.ExportStatusPending, result.Status)
	assert.False(t, result.IncludeFileData)
	assert.Equal(t, "Test get export", result.Description)
}

func TestExportRegistry_Update(t *testing.T) {
	db := PrepareTestDB()

	registry := NewExportRegistry(db)

	// Create an export first
	export := models.Export{
		Type:            models.ExportTypeLocations,
		Status:          models.ExportStatusPending,
		IncludeFileData: false,
		Description:     "Test update export",
		CreatedDate:     models.PDate{},
	}

	created, err := registry.Create(context.Background(), export)
	require.NoError(t, err)

	// Update the export
	created.Status = models.ExportStatusCompleted
	created.FilePath = "/path/to/export.xml"

	result, err := registry.Update(context.Background(), *created)
	require.NoError(t, err)
	assert.Equal(t, models.ExportStatusCompleted, result.Status)
	assert.Equal(t, "/path/to/export.xml", result.FilePath)
}

func TestExportRegistry_List(t *testing.T) {
	db := PrepareTestDB()

	registry := NewExportRegistry(db)

	// Create multiple exports
	export1 := models.Export{
		Type:        models.ExportTypeFullDatabase,
		Status:      models.ExportStatusPending,
		Description: "Export 1",
		CreatedDate: models.PDate{},
	}

	export2 := models.Export{
		Type:        models.ExportTypeCommodities,
		Status:      models.ExportStatusCompleted,
		Description: "Export 2",
		CreatedDate: models.PDate{},
	}

	_, err := registry.Create(context.Background(), export1)
	require.NoError(t, err)

	_, err = registry.Create(context.Background(), export2)
	require.NoError(t, err)

	// List exports
	exports, err := registry.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, exports, 2)
}

func TestExportRegistry_Delete(t *testing.T) {
	db := PrepareTestDB()

	registry := NewExportRegistry(db)

	// Create an export first
	export := models.Export{
		Type:        models.ExportTypeAreas,
		Status:      models.ExportStatusFailed,
		Description: "Test delete export",
		CreatedDate: models.PDate{},
	}

	created, err := registry.Create(context.Background(), export)
	require.NoError(t, err)

	// Delete the export
	err = registry.Delete(context.Background(), created.GetID())
	require.NoError(t, err)

	// Verify it's deleted
	_, err = registry.Get(context.Background(), created.GetID())
	assert.Error(t, err)
}

func TestExportRegistry_Count(t *testing.T) {
	db := PrepareTestDB()

	registry := NewExportRegistry(db)

	// Initially should be 0
	count, err := registry.Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Create an export
	export := models.Export{
		Type:        models.ExportTypeFullDatabase,
		Status:      models.ExportStatusPending,
		Description: "Test count export",
		CreatedDate: models.PDate{},
	}

	_, err = registry.Create(context.Background(), export)
	require.NoError(t, err)

	// Now should be 1
	count, err = registry.Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
