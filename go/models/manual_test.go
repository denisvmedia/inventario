package models_test

import (
	"context"
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestManual_Validate(t *testing.T) {
	c := qt.New(t)

	manual := &models.Manual{}
	err := manual.Validate()
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorIs, models.ErrMustUseValidateWithContext)
}

func TestManual_ValidateWithContext_HappyPath(t *testing.T) {
	t.Run("valid manual", func(t *testing.T) {
		c := qt.New(t)

		manual := models.Manual{
			CommodityID: "commodity-123",
			File: &models.File{
				Path:         "test-manual",
				OriginalPath: "test-manual.pdf",
				Ext:          ".pdf",
				MIMEType:     "application/pdf",
			},
		}

		ctx := context.Background()
		err := manual.ValidateWithContext(ctx)
		c.Assert(err, qt.IsNil)
	})
}

func TestManual_ValidateWithContext_UnhappyPaths(t *testing.T) {
	testCases := []struct {
		name          string
		manual        models.Manual
		errorContains string
	}{
		{
			name:          "missing commodity_id",
			manual:        models.Manual{File: &models.File{Path: "test", OriginalPath: "test.pdf", Ext: ".pdf", MIMEType: "application/pdf"}},
			errorContains: "commodity_id: cannot be blank",
		},
		{
			name:          "missing file",
			manual:        models.Manual{CommodityID: "commodity-123"},
			errorContains: "File: cannot be blank",
		},
		{
			name:          "empty manual",
			manual:        models.Manual{},
			errorContains: "File: cannot be blank",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			ctx := context.Background()
			err := tc.manual.ValidateWithContext(ctx)
			c.Assert(err, qt.Not(qt.IsNil))
			c.Assert(err.Error(), qt.Contains, tc.errorContains)
		})
	}
}

func TestManual_JSONMarshaling(t *testing.T) {
	c := qt.New(t)

	// Create a manual with all fields populated
	manual := models.Manual{
		EntityID: models.EntityID{
			ID: "manual-123",
		},
		CommodityID: "commodity-123",
		File: &models.File{
			Path:         "test-manual",
			OriginalPath: "test-manual.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}

	// Marshal the manual to JSON
	jsonData, err := json.Marshal(manual)
	c.Assert(err, qt.IsNil)

	// Unmarshal the JSON back to a manual
	var unmarshaledManual models.Manual
	err = json.Unmarshal(jsonData, &unmarshaledManual)
	c.Assert(err, qt.IsNil)

	// Verify that the unmarshaled manual matches the original
	c.Assert(unmarshaledManual.ID, qt.Equals, manual.ID)
	c.Assert(unmarshaledManual.CommodityID, qt.Equals, manual.CommodityID)
	c.Assert(unmarshaledManual.File.Path, qt.Equals, manual.File.Path)
	c.Assert(unmarshaledManual.File.OriginalPath, qt.Equals, manual.File.OriginalPath)
	c.Assert(unmarshaledManual.File.Ext, qt.Equals, manual.File.Ext)
	c.Assert(unmarshaledManual.File.MIMEType, qt.Equals, manual.File.MIMEType)
}

func TestManual_IDable(t *testing.T) {
	c := qt.New(t)

	// Create a manual
	manual := models.Manual{
		EntityID: models.EntityID{
			ID: "manual-123",
		},
		CommodityID: "commodity-123",
		File: &models.File{
			Path:         "test-manual",
			OriginalPath: "test-manual.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}

	// Test GetID
	c.Assert(manual.GetID(), qt.Equals, "manual-123")

	// Test SetID
	manual.SetID("new-manual-id")
	c.Assert(manual.GetID(), qt.Equals, "new-manual-id")
}
