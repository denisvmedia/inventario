// jsonapi/invoices.go

package jsonapi

import (
	"net/http"

	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/models"
)

// InvoiceResponse is an object that holds invoice information.
type InvoiceResponse struct {
	HTTPStatusCode int `json:"-"` // HTTP response status code

	ID         string         `json:"id"`
	Type       string         `json:"type" example:"invoices" enums:"invoices"`
	Attributes models.Invoice `json:"attributes"`
}

// NewInvoiceResponse creates a new InvoiceResponse instance.
func NewInvoiceResponse(invoice *models.Invoice) *InvoiceResponse {
	return &InvoiceResponse{
		ID:         invoice.ID,
		Type:       "invoices",
		Attributes: *invoice,
	}
}

// WithStatusCode sets the HTTP response status code for the InvoiceResponse.
func (ir *InvoiceResponse) WithStatusCode(statusCode int) *InvoiceResponse {
	tmp := *ir
	tmp.HTTPStatusCode = statusCode
	return &tmp
}

// Render renders the InvoiceResponse as an HTTP response.
func (ir *InvoiceResponse) Render(_w http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(ir.HTTPStatusCode, http.StatusOK))
	return nil
}

// InvoicesMeta is a meta information for InvoicesResponse.
type InvoicesMeta struct {
	Invoices int `json:"invoices" example:"1" format:"int64"`
}

// InvoicesResponse is an object that holds a list of invoice information.
type InvoicesResponse struct {
	Data []*models.Invoice `json:"data"`
	Meta InvoicesMeta      `json:"meta"`
}

// NewInvoicesResponse creates a new InvoicesResponse instance.
func NewInvoicesResponse(invoices []*models.Invoice, total int) *InvoicesResponse {
	return &InvoicesResponse{
		Data: invoices,
		Meta: InvoicesMeta{Invoices: total},
	}
}

// Render renders the InvoicesResponse as an HTTP response.
func (*InvoicesResponse) Render(_w http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}
