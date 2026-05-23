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
	Fields     map[string]CommodityScanFieldGuess `json:"fields"`
	Warnings   []CommodityScanWarning             `json:"warnings,omitempty"`
	Provider   string                             `json:"provider,omitempty"`
	UsedTokens int                                `json:"used_tokens,omitempty"`
	LatencyMS  int64                              `json:"latency_ms,omitempty"`
}

// CommodityScanFieldGuess is the per-field extraction value plus a
// 0..1 confidence score. value can be a string, number, or []string
// depending on the field name; the FE switches on the field key.
type CommodityScanFieldGuess struct {
	Value      any     `json:"value" swaggertype:"object"`
	Confidence float64 `json:"confidence"`
}

// CommodityScanWarning is a non-fatal note attached to a scan
// response. code is a stable identifier the FE branches on.
type CommodityScanWarning struct {
	Code   string `json:"code" example:"low_confidence" enums:"low_confidence,unreadable_serial,ambiguous_price,currency_inferred,no_photo_text"`
	Field  string `json:"field,omitempty"`
	Detail string `json:"detail,omitempty"`
}
