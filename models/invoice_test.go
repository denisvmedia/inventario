package models_test

import (
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestInvoice_Validate_HappyPath(t *testing.T) {
	t.Run("valid invoice", func(t *testing.T) {
		c := qt.New(t)

		invoice := models.Invoice{
			CommodityID: "commodity-123",
			File: &models.File{
				Path:         "test-invoice",
				OriginalPath: "test-invoice.pdf",
				Ext:          ".pdf",
				MIMEType:     "application/pdf",
			},
		}

		err := invoice.Validate()
		c.Assert(err, qt.IsNil)
	})
}

func TestInvoice_Validate_UnhappyPaths(t *testing.T) {
	testCases := []struct {
		name          string
		invoice       models.Invoice
		errorContains string
	}{
		{
			name:          "missing commodity_id",
			invoice:       models.Invoice{File: &models.File{Path: "test", OriginalPath: "test.pdf", Ext: ".pdf", MIMEType: "application/pdf"}},
			errorContains: "commodity_id: cannot be blank",
		},
		{
			name:          "missing file",
			invoice:       models.Invoice{CommodityID: "commodity-123"},
			errorContains: "File: cannot be blank",
		},
		{
			name:          "empty invoice",
			invoice:       models.Invoice{},
			errorContains: "File: cannot be blank",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			err := tc.invoice.Validate()
			c.Assert(err, qt.Not(qt.IsNil))
			c.Assert(err.Error(), qt.Contains, tc.errorContains)
		})
	}
}

func TestInvoice_JSONMarshaling(t *testing.T) {
	c := qt.New(t)

	// Create an invoice with all fields populated
	invoice := models.Invoice{
		EntityID: models.EntityID{
			ID: "invoice-123",
		},
		CommodityID: "commodity-123",
		File: &models.File{
			Path:         "test-invoice",
			OriginalPath: "test-invoice.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}

	// Marshal the invoice to JSON
	jsonData, err := json.Marshal(invoice)
	c.Assert(err, qt.IsNil)

	// Unmarshal the JSON back to an invoice
	var unmarshaledInvoice models.Invoice
	err = json.Unmarshal(jsonData, &unmarshaledInvoice)
	c.Assert(err, qt.IsNil)

	// Verify that the unmarshaled invoice matches the original
	c.Assert(unmarshaledInvoice.ID, qt.Equals, invoice.ID)
	c.Assert(unmarshaledInvoice.CommodityID, qt.Equals, invoice.CommodityID)
	c.Assert(unmarshaledInvoice.File.Path, qt.Equals, invoice.File.Path)
	c.Assert(unmarshaledInvoice.File.OriginalPath, qt.Equals, invoice.File.OriginalPath)
	c.Assert(unmarshaledInvoice.File.Ext, qt.Equals, invoice.File.Ext)
	c.Assert(unmarshaledInvoice.File.MIMEType, qt.Equals, invoice.File.MIMEType)
}

func TestInvoice_IDable(t *testing.T) {
	c := qt.New(t)

	// Create an invoice
	invoice := models.Invoice{
		EntityID: models.EntityID{
			ID: "invoice-123",
		},
		CommodityID: "commodity-123",
		File: &models.File{
			Path:         "test-invoice",
			OriginalPath: "test-invoice.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}

	// Test GetID
	c.Assert(invoice.GetID(), qt.Equals, "invoice-123")

	// Test SetID
	invoice.SetID("new-invoice-id")
	c.Assert(invoice.GetID(), qt.Equals, "new-invoice-id")
}
