package jsonapi

// CommodityScanResponse is the JSON:API envelope for the AI vision
// photo-scan endpoint (issue #1720). The Type "commodity_scan" is
// deliberately distinct from "commodities" so the FE knows the body
// is a *suggestion* rather than a persisted commodity.
type CommodityScanResponse struct {
	Data CommodityScanResource `json:"data"`
}

// CommodityScanResource holds the typed scan attributes.
type CommodityScanResource struct {
	Type       string                  `json:"type" example:"commodity_scan" enums:"commodity_scan"`
	Attributes CommodityScanAttributes `json:"attributes"`
}

// CommodityScanAttributes is the result body. fields is keyed by
// canonical field name (name, short_name, type, original_price,
// original_price_currency, serial_number, urls, purchase_date,
// warranty_expires_at, comments, tags, document_type); items carries one
// such field set per product on a multi-product scan; files carries the
// suggestion for each input file; warnings is the optional non-fatal note
// list.
//
// fields.document_type classifies the scan as a whole (invoice, manual,
// receipt, warranty or other). files is the per-file source: when the model
// gives no scan-level value it is the document type of the first document
// among the files, or other when they are all photos or other files.
type CommodityScanAttributes struct {
	// Fields are the values read, keyed by field name. Its document_type
	// classifies the scan as a whole; files is the per-file source, and
	// document_type is derived from it when the model gives none.
	Fields map[string]CommodityScanFieldGuess `json:"fields"`
	// CustomFields are the values read for the group's custom fields
	// (#2803), keyed by field ID, in the form an item write takes them: a
	// string for text, long text, URL, date and a select field's option ID,
	// a number, or a boolean. Absent when the scan read none.
	CustomFields map[string]CommodityScanFieldGuess `json:"custom_fields,omitempty"`
	// Items is present only when the source described more than one
	// distinct product; one entry per product, most prominent first. The
	// FE renders a chooser. Empty/absent for a single product.
	Items []CommodityScanItem `json:"items,omitempty"`
	// Files holds one suggestion per input file the model said something
	// usable about, ordered by index; a file without one has no entry.
	// Absent when there are none.
	Files      []CommodityScanFileGuess `json:"files,omitempty"`
	Warnings   []CommodityScanWarning   `json:"warnings,omitempty"`
	UsedTokens int                      `json:"used_tokens,omitempty"`
	LatencyMS  int64                    `json:"latency_ms,omitempty"`
}

// CommodityScanItem is one candidate product in a multi-product scan; its
// fields mirror CommodityScanAttributes.Fields.
type CommodityScanItem struct {
	Fields       map[string]CommodityScanFieldGuess `json:"fields"`
	CustomFields map[string]CommodityScanFieldGuess `json:"custom_fields,omitempty"`
}

// CommodityScanFieldGuess is the per-field extraction value plus a
// 0..1 confidence score. The polymorphic shape of value is documented
// in this comment because swag (Swagger 2.0) does not have first-class
// support for oneOf / union types on a struct field, and openapi-typescript
// renders a property-less `{type: object}` schema as Record<string, never>
// (a closed, indexable-but-empty object). The FE explicitly switches
// on the field key and casts value into the right concrete type — both
// sides are under our control, so we accept the typing fudge for now
// rather than emitting a misleading `additionalProperties:{type:string}`
// override (the value is not always an object, and not always strings):
//
//   - name, short_name, brand, model, type, serial_number, comments,
//     original_price_currency, purchase_date,
//     warranty_expires_at (both YYYY-MM-DD): string
//   - original_price: number (decimal)
//   - urls, tags: string[]
type CommodityScanFieldGuess struct {
	// Value is polymorphic — see the type-level doc comment.
	Value      any     `json:"value" swaggertype:"object"`
	Confidence float64 `json:"confidence"`
}

// CommodityScanFileGuess is the suggestion for one input file of a scan.
// Each part is absent when the model gave nothing usable for it.
type CommodityScanFileGuess struct {
	// Index is the file's position among the photos parts of the request,
	// counted from 0. An empty part the server skipped still counts.
	Index        int                             `json:"index" example:"0" validate:"required"`
	DocumentType *CommodityScanDocumentTypeGuess `json:"document_type,omitempty"`
	Title        *CommodityScanTitleGuess        `json:"title,omitempty"`
}

// CommodityScanDocumentTypeGuess is what a file is: a document of one of
// the kinds, or a photo of the product itself.
type CommodityScanDocumentTypeGuess struct {
	Value      string  `json:"value" example:"invoice" enums:"invoice,receipt,manual,warranty,photo,other"`
	Confidence float64 `json:"confidence" example:"0.9"`
}

// CommodityScanTitleGuess is a short readable title for a file, in the
// language of the document and at most 120 characters.
type CommodityScanTitleGuess struct {
	Value      string  `json:"value" example:"Bosch invoice, March 2024" maxLength:"120"`
	Confidence float64 `json:"confidence" example:"0.8"`
}

// CommodityScanWarning is a non-fatal note attached to a scan
// response. code is a stable identifier the FE branches on.
type CommodityScanWarning struct {
	Code   string `json:"code" example:"low_confidence" enums:"low_confidence,unreadable_serial,ambiguous_price,currency_inferred,no_photo_text,multiple_items"`
	Field  string `json:"field,omitempty"`
	Detail string `json:"detail,omitempty"`
}
