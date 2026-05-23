package models_test

import (
	"context"
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestFile_Validate(t *testing.T) {
	c := qt.New(t)

	file := &models.File{}
	err := file.Validate()
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Equals, "must use validate with context")
}

func TestFile_ValidateWithContext_HappyPath(t *testing.T) {
	t.Run("fully populated file", func(t *testing.T) {
		c := qt.New(t)

		file := models.File{
			Path:         "test-file",
			OriginalPath: "test-file.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		}

		ctx := context.Background()
		err := file.ValidateWithContext(ctx)
		c.Assert(err, qt.IsNil)
	})

	// Post-#1779 createFile writes an empty File until the upload
	// handler populates it. Validation must accept the empty literal so
	// the metadata-only POST /files path still goes through. The shape
	// invariants live on FileEntity (Type / Category / linked-entity
	// invariants), not on the File sub-struct itself.
	t.Run("empty file passes (placeholder before upload)", func(t *testing.T) {
		c := qt.New(t)

		file := models.File{}

		ctx := context.Background()
		err := file.ValidateWithContext(ctx)
		c.Assert(err, qt.IsNil)
	})
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
}
