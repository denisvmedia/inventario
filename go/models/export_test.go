package models_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestExportStatus_IsValid(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		status models.ExportStatus
		valid  bool
	}{
		{models.ExportStatusPending, true},
		{models.ExportStatusInProgress, true},
		{models.ExportStatusCompleted, true},
		{models.ExportStatusFailed, true},
		{"invalid", false},
		{"", false},
	}

	for _, test := range tests {
		t.Run(string(test.status), func(t *testing.T) {
			c.Assert(test.status.IsValid(), qt.Equals, test.valid)
		})
	}
}

func TestExportType_IsValid(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		exportType models.ExportType
		valid      bool
	}{
		{models.ExportTypeFullDatabase, true},
		{models.ExportTypeSelectedItems, true},
		{models.ExportTypeLocations, true},
		{models.ExportTypeAreas, true},
		{models.ExportTypeCommodities, true},
		{"invalid", false},
		{"", false},
	}

	for _, test := range tests {
		t.Run(string(test.exportType), func(t *testing.T) {
			c.Assert(test.exportType.IsValid(), qt.Equals, test.valid)
		})
	}
}

func TestExport_ValidateWithContext(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Valid export
	createdDate := models.Date("2023-01-01")
	validExport := &models.Export{
		Type:            models.ExportTypeFullDatabase,
		Status:          models.ExportStatusPending,
		IncludeFileData: true,
		CreatedDate:     &createdDate,
		Description:     "Test export",
	}

	err := validExport.ValidateWithContext(ctx)
	c.Assert(err, qt.IsNil)

	// Invalid export - empty type
	invalidExport := &models.Export{
		Type:        "",
		Status:      models.ExportStatusPending,
		CreatedDate: &createdDate,
	}

	err = invalidExport.ValidateWithContext(ctx)
	c.Assert(err, qt.IsNotNil)

	// Invalid export - selected items without IDs
	invalidSelectedExport := &models.Export{
		Type:          models.ExportTypeSelectedItems,
		Status:        models.ExportStatusPending,
		CreatedDate:   &createdDate,
		SelectedItems: models.ValuerSlice[models.ExportSelectedItem]{},
	}

	err = invalidSelectedExport.ValidateWithContext(ctx)
	c.Assert(err, qt.IsNotNil)

	// Valid export - selected items with IDs
	validSelectedExport := &models.Export{
		Type:        models.ExportTypeSelectedItems,
		Status:      models.ExportStatusPending,
		CreatedDate: &createdDate,
		SelectedItems: models.ValuerSlice[models.ExportSelectedItem]{
			{ID: "id1", Type: models.ExportSelectedItemTypeCommodity},
			{ID: "id2", Type: models.ExportSelectedItemTypeLocation},
		},
	}

	err = validSelectedExport.ValidateWithContext(ctx)
	c.Assert(err, qt.IsNil)
}
