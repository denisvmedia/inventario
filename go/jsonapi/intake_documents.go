package jsonapi

import (
	"net/http"

	"github.com/go-chi/render"

	"go.5x5.cz/inventario/models"
)

// IntakeDocumentView is a document another system delivered (#2809) with
// what became of it: "pending" while no draft is made yet, "draft" while the
// draft waits in the inbox, "item" once it became an item, "removed" when the
// draft was discarded or the item deleted.
type IntakeDocumentView struct {
	*models.IntakeDocument
	State models.IntakeDocumentState `json:"state" example:"draft" enums:"pending,draft,item,removed"`
}

// IntakeDocumentResponseData is the resource object of a delivered document.
type IntakeDocumentResponseData struct {
	ID         string             `json:"id"`
	Type       string             `json:"type" example:"intake_documents" enums:"intake_documents"`
	Attributes IntakeDocumentView `json:"attributes"`
}

func newIntakeDocumentData(doc *models.IntakeDocument, state models.IntakeDocumentState) IntakeDocumentResponseData {
	return IntakeDocumentResponseData{
		ID:         doc.ID,
		Type:       "intake_documents",
		Attributes: IntakeDocumentView{IntakeDocument: doc, State: state},
	}
}

// IntakeDocumentResponse is the JSON:API envelope for one delivered document.
type IntakeDocumentResponse struct {
	HTTPStatusCode int                        `json:"-"`
	Data           IntakeDocumentResponseData `json:"data"`
}

func NewIntakeDocumentResponse(doc *models.IntakeDocument, state models.IntakeDocumentState) *IntakeDocumentResponse {
	return &IntakeDocumentResponse{Data: newIntakeDocumentData(doc, state)}
}

func (rd *IntakeDocumentResponse) WithStatusCode(code int) *IntakeDocumentResponse {
	tmp := *rd
	tmp.HTTPStatusCode = code
	return &tmp
}

func (rd *IntakeDocumentResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(rd.HTTPStatusCode, http.StatusOK))
	return nil
}

// IntakeDocumentsMeta is the count block of a list of delivered documents.
type IntakeDocumentsMeta struct {
	Documents int `json:"documents" example:"1" format:"int64"`
}

// IntakeDocumentsResponse lists delivered documents.
type IntakeDocumentsResponse struct {
	Data []IntakeDocumentResponseData `json:"data"`
	Meta IntakeDocumentsMeta          `json:"meta"`
}

// NewIntakeDocumentsResponse builds the list; states holds the state of each
// document by id.
func NewIntakeDocumentsResponse(docs []*models.IntakeDocument, states map[string]models.IntakeDocumentState) *IntakeDocumentsResponse {
	data := make([]IntakeDocumentResponseData, 0, len(docs))
	for _, doc := range docs {
		data = append(data, newIntakeDocumentData(doc, states[doc.ID]))
	}
	return &IntakeDocumentsResponse{Data: data, Meta: IntakeDocumentsMeta{Documents: len(data)}}
}

func (*IntakeDocumentsResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// IntakeDeliveryRequest documents the fields of POST /g/{groupSlug}/intake.
// The endpoint reads them as a flat JSON object, as form fields, or as the
// fields of a multipart form with the file in a part named "file", so another
// system's webhook can post them without building a JSON:API envelope.
type IntakeDeliveryRequest struct {
	// Title becomes the draft's name.
	Title string `json:"title" example:"Invoice 2024-117" validate:"required"`
	// SourceSystem names the other system; with SourceInstance and
	// DocumentID it identifies the document in the group.
	SourceSystem string `json:"source_system" example:"paperless-ngx" validate:"required"`
	// SourceInstance tells installations of one system apart. Optional.
	SourceInstance string `json:"source_instance" example:"home"`
	// DocumentID is the document's id in the other system.
	DocumentID string `json:"document_id" example:"117" validate:"required"`
	// SourceURL links to the document there: absolute http or https. It is
	// stored and shown, never fetched.
	SourceURL string `json:"source_url" example:"https://paperless.example.com/documents/117/"`
	// SourceName is how the other system is shown. Optional.
	SourceName string `json:"source_name" example:"Paperless-ngx"`
	// Seller, PurchaseDate (YYYY-MM-DD), Amount and Currency are the
	// purchase data the other system knows. Each is optional; what is left
	// out stays unknown. Amount may start with a currency code ("EUR12.30").
	Seller       string `json:"seller" example:"Hardware Store"`
	PurchaseDate string `json:"purchase_date" example:"2024-03-14"`
	Amount       string `json:"amount" example:"129.90"`
	Currency     string `json:"currency" example:"EUR"`
}
