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
// comments); warnings is the optional non-fatal note list.
type CommodityScanAttributes struct {
	Fields map[string]CommodityScanFieldGuess `json:"fields"`
	// Items is present only when the source described more than one
	// distinct product; one entry per product, most prominent first. The
	// FE renders a chooser. Empty/absent for a single product.
	Items      []CommodityScanItem    `json:"items,omitempty"`
	Warnings   []CommodityScanWarning `json:"warnings,omitempty"`
	UsedTokens int                    `json:"used_tokens,omitempty"`
	LatencyMS  int64                  `json:"latency_ms,omitempty"`
}

// CommodityScanItem is one candidate product in a multi-product scan; its
// fields mirror CommodityScanAttributes.Fields.
type CommodityScanItem struct {
	Fields map[string]CommodityScanFieldGuess `json:"fields"`
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
//   - name, short_name, type, serial_number, comments,
//     original_price_currency, purchase_date (YYYY-MM-DD): string
//   - original_price: number (decimal)
//   - urls: string[]
type CommodityScanFieldGuess struct {
	// Value is polymorphic — see the type-level doc comment.
	Value      any     `json:"value" swaggertype:"object"`
	Confidence float64 `json:"confidence"`
}

// CommodityScanWarning is a non-fatal note attached to a scan
// response. code is a stable identifier the FE branches on.
type CommodityScanWarning struct {
	Code   string `json:"code" example:"low_confidence" enums:"low_confidence,unreadable_serial,ambiguous_price,currency_inferred,no_photo_text,multiple_items"`
	Field  string `json:"field,omitempty"`
	Detail string `json:"detail,omitempty"`
}
