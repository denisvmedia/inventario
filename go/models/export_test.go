package models

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExportStatus_IsValid(t *testing.T) {
	tests := []struct {
		status ExportStatus
		valid  bool
	}{
		{ExportStatusPending, true},
		{ExportStatusInProgress, true},
		{ExportStatusCompleted, true},
		{ExportStatusFailed, true},
		{"invalid", false},
		{"", false},
	}

	for _, test := range tests {
		t.Run(string(test.status), func(t *testing.T) {
			assert.Equal(t, test.valid, test.status.IsValid())
		})
	}
}

func TestExportType_IsValid(t *testing.T) {
	tests := []struct {
		exportType ExportType
		valid      bool
	}{
		{ExportTypeFullDatabase, true},
		{ExportTypeSelectedItems, true},
		{ExportTypeLocations, true},
		{ExportTypeAreas, true},
		{ExportTypeCommodities, true},
		{"invalid", false},
		{"", false},
	}

	for _, test := range tests {
		t.Run(string(test.exportType), func(t *testing.T) {
			assert.Equal(t, test.valid, test.exportType.IsValid())
		})
	}
}

func TestExport_ValidateWithContext(t *testing.T) {
	ctx := context.Background()

	// Valid export
	createdDate := Date("2023-01-01")
	validExport := &Export{
		Type:            ExportTypeFullDatabase,
		Status:          ExportStatusPending,
		IncludeFileData: true,
		CreatedDate:     &createdDate,
		Description:     "Test export",
	}

	err := validExport.ValidateWithContext(ctx)
	assert.NoError(t, err)

	// Invalid export - empty type
	invalidExport := &Export{
		Type:        "",
		Status:      ExportStatusPending,
		CreatedDate: &createdDate,
	}

	err = invalidExport.ValidateWithContext(ctx)
	assert.Error(t, err)

	// Invalid export - selected items without IDs
	invalidSelectedExport := &Export{
		Type:            ExportTypeSelectedItems,
		Status:          ExportStatusPending,
		CreatedDate:     &createdDate,
		SelectedItemIDs: ValuerSlice[string]{},
	}

	err = invalidSelectedExport.ValidateWithContext(ctx)
	assert.Error(t, err)

	// Valid export - selected items with IDs
	validSelectedExport := &Export{
		Type:            ExportTypeSelectedItems,
		Status:          ExportStatusPending,
		CreatedDate:     &createdDate,
		SelectedItemIDs: ValuerSlice[string]{"id1", "id2"},
	}

	err = validSelectedExport.ValidateWithContext(ctx)
	assert.NoError(t, err)
}