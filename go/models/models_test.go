package models_test

import (
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestFile_Validate_HappyPath(t *testing.T) {
	t.Run("valid file", func(t *testing.T) {
		c := qt.New(t)

		file := models.File{
			Path:         "test-file",
			OriginalPath: "test-file.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		}

		err := file.Validate()
		c.Assert(err, qt.IsNil)
	})
}

func TestFile_Validate_UnhappyPaths(t *testing.T) {
	testCases := []struct {
		name          string
		file          models.File
		errorContains string
	}{
		{
			name:          "missing path",
			file:          models.File{OriginalPath: "test.pdf", Ext: ".pdf", MIMEType: "application/pdf"},
			errorContains: "path: cannot be blank",
		},
		{
			name:          "missing original path",
			file:          models.File{Path: "test", Ext: ".pdf", MIMEType: "application/pdf"},
			errorContains: "original_path: cannot be blank",
		},
		{
			name:          "missing extension",
			file:          models.File{Path: "test", OriginalPath: "test.pdf", MIMEType: "application/pdf"},
			errorContains: "ext: cannot be blank",
		},
		{
			name:          "missing MIME type",
			file:          models.File{Path: "test", OriginalPath: "test.pdf", Ext: ".pdf"},
			errorContains: "mime_type: cannot be blank",
		},
		{
			name:          "empty file",
			file:          models.File{},
			errorContains: "path: cannot be blank",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			err := tc.file.Validate()
			c.Assert(err, qt.Not(qt.IsNil))
			c.Assert(err.Error(), qt.Contains, tc.errorContains)
		})
	}
}

func TestFile_JSONMarshaling(t *testing.T) {
	c := qt.New(t)

	// Create a file with all fields populated
	file := models.File{
		Path:         "test-file",
		OriginalPath: "test-file.pdf",
		Ext:          ".pdf",
		MIMEType:     "application/pdf",
	}

	// Marshal the file to JSON
	jsonData, err := json.Marshal(file)
	c.Assert(err, qt.IsNil)

	// Unmarshal the JSON back to a file
	var unmarshaledFile models.File
	err = json.Unmarshal(jsonData, &unmarshaledFile)
	c.Assert(err, qt.IsNil)

	// Verify that the unmarshaled file matches the original
	c.Assert(unmarshaledFile.Path, qt.Equals, file.Path)
	c.Assert(unmarshaledFile.OriginalPath, qt.Equals, file.OriginalPath)
	c.Assert(unmarshaledFile.Ext, qt.Equals, file.Ext)
	c.Assert(unmarshaledFile.MIMEType, qt.Equals, file.MIMEType)
}

func TestEntityID_IDable(t *testing.T) {
	c := qt.New(t)

	// Create an EntityID
	entityID := models.EntityID{
		ID: "entity-123",
	}

	// Test GetID
	c.Assert(entityID.GetID(), qt.Equals, "entity-123")

	// Test SetID
	entityID.SetID("new-entity-id")
	c.Assert(entityID.GetID(), qt.Equals, "new-entity-id")
}

func TestWithID(t *testing.T) {
	c := qt.New(t)

	// Test WithID with a Location
	location := &models.Location{
		Name:    "Test Location",
		Address: "123 Test Street",
	}

	locationWithID := models.WithID("location-123", location)
	c.Assert(locationWithID.GetID(), qt.Equals, "location-123")

	// Test WithID with a Manual
	manual := &models.Manual{
		CommodityID: "commodity-123",
		File: &models.File{
			Path:         "test-manual",
			OriginalPath: "test-manual.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}

	manualWithID := models.WithID("manual-123", manual)
	c.Assert(manualWithID.GetID(), qt.Equals, "manual-123")
}
