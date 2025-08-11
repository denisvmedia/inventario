package models_test

import (
	"context"
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestInvoice_Validate(t *testing.T) {
	c := qt.New(t)

	invoice := &models.Invoice{}
	err := invoice.Validate()
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorIs, models.ErrMustUseValidateWithContext)
}

func TestInvoice_ValidateWithContext_HappyPath(t *testing.T) {
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

		ctx := context.Background()
		err := invoice.ValidateWithContext(ctx)
		c.Assert(err, qt.IsNil)
	})
}

func TestInvoice_ValidateWithContext_UnhappyPaths(t *testing.T) {
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

			ctx := context.Background()
			err := tc.invoice.ValidateWithContext(ctx)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.errorContains)
		})
	}
}

func TestInvoice_JSONMarshaling(t *testing.T) {
	c := qt.New(t)

	// Create an invoice with all fields populated
	invoice := models.Invoice{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{
				ID: "invoice-123",
			},
			TenantID: "test-tenant",
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
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{
				ID: "invoice-123",
			},
			TenantID: "test-tenant",
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
