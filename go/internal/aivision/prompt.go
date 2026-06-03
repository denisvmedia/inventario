package aivision

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SystemPrompt is the role/system instruction sent to every vendor. It
// is intentionally short — vendors trim very long system prompts and a
// concise instruction reduces token spend on every call.
const SystemPrompt = "You are an assistant that extracts structured product information from photos and documents of physical items.\n" +
	"You will receive 1 to 5 inputs — photos and/or PDFs. They may show a product (front, back, label, packaging, etc.) " +
	"or a purchase document such as a receipt, invoice, or product manual. A receipt/invoice may itself be a PDF, a scan, " +
	"an export, or a photo (jpg/png/heic) — handle it the same regardless of format.\n" +
	"Most inputs describe ONE product — extract it into \"fields\".\n" +
	"A receipt or invoice often lists SEVERAL purchased products. Whenever more than one distinct product is present, " +
	"return EACH purchased product as its own entry in \"items\" (most prominent / most expensive first), each shaped as " +
	"{\"fields\": {...}}, and set the top-level \"fields\" to the first entry. IGNORE non-product lines: subtotals, " +
	"taxes/VAT, shipping, discounts, fees, deposits, and totals. Omit \"items\" entirely when there is only one product.\n" +
	"For each product (including each invoice line item), read its purchase price, currency, and purchase date; " +
	"put the seller/vendor/store name (there is no dedicated seller field) into \"comments\".\n" +
	"Classify \"type\" as EXACTLY one of the allowed values in the schema enum; omit it if none clearly fits.\n" +
	"Keep \"short_name\" a concise label of at most 40 characters.\n" +
	"If a warranty period or expiry is shown, set \"warranty_expires_at\" to the warranty END date (YYYY-MM-DD); " +
	"if only a duration like \"2 years\" is given and the purchase date is known, add the duration to the purchase date.\n" +
	"Return ONE JSON object that matches the requested schema EXACTLY. Do not include any prose, markdown, or extra keys.\n" +
	"For each field, ALWAYS include a \"confidence\" score between 0.0 and 1.0 reflecting how sure you are.\n" +
	"Omit fields you have NO evidence for rather than guessing — null/empty is preferred over hallucinated values."

// UserPromptHeader is the literal text prepended to the multimodal user
// turn. It tells the model what task to perform; the actual schema is
// delivered via the per-vendor structured output channel (Anthropic
// tool-use schema, OpenAI response_format json_schema).
func UserPromptHeader(req ScanRequest) string {
	var b strings.Builder
	b.WriteString("Extract the product information from the attached photo(s) and/or document(s).\n")
	if req.PreferredCurrencyCode != "" {
		fmt.Fprintf(&b, "If a price is visible without a clear currency symbol, prefer currency code %q.\n", req.PreferredCurrencyCode)
	}
	if req.HintFromUser != "" {
		fmt.Fprintf(&b, "User hint: %s\n", req.HintFromUser)
	}
	b.WriteString("\nReturn the JSON object now.")
	return b.String()
}

// ResponseSchema builds the JSON Schema (draft-07 compatible subset both
// vendors accept) that constrains the structured output. Keeping it
// shared between providers prevents drift — the FE has to handle exactly
// one response shape regardless of the vendor selected at boot.
//
// The schema is deliberately permissive on confidence (number, 0..1
// inclusive) and on optional fields. Required: nothing — every field is
// optional so the model is free to omit anything it can't read.
func ResponseSchema() map[string]any {
	fieldGuessString := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"value":      map[string]any{"type": "string"},
			"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		},
		"required":             []string{"value", "confidence"},
		"additionalProperties": false,
	}
	fieldGuessNumber := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"value":      map[string]any{"type": "number"},
			"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		},
		"required":             []string{"value", "confidence"},
		"additionalProperties": false,
	}
	fieldGuessStringArray := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"value":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		},
		"required":             []string{"value", "confidence"},
		"additionalProperties": false,
	}
	// fieldGuessStringMax mirrors fieldGuessString but caps the value
	// length, steering the model toward the form's limit (the FE still
	// truncates defensively). Used for short_name (40 chars).
	fieldGuessStringMax := func(maxLen int) map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"value":      map[string]any{"type": "string", "maxLength": maxLen},
				"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
			},
			"required":             []string{"value", "confidence"},
			"additionalProperties": false,
		}
	}
	// fieldGuessEnum constrains the value to a closed set of strings so the
	// model's "type" guess stays inside the categories the FE's isKnownType
	// accepts — otherwise a valid-but-free-form guess (e.g. "laptop") is
	// silently dropped instead of pre-filling.
	fieldGuessEnum := func(values []string) map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"value":      map[string]any{"type": "string", "enum": values},
				"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
			},
			"required":             []string{"value", "confidence"},
			"additionalProperties": false,
		}
	}

	// commodityTypes mirrors models.CommodityType* and the FE COMMODITY_TYPES
	// constant. Hardcoded so this vendor-neutral leaf package stays free of a
	// domain-model dependency; the FE's isKnownType is the authoritative gate,
	// so any drift only costs a dropped type guess, never a bad write.
	commodityTypes := []string{"white_goods", "electronics", "equipment", "furniture", "clothes", "other"}

	fields := map[string]any{
		FieldNameName:                  fieldGuessString,
		FieldNameShortName:             fieldGuessStringMax(40),
		FieldNameType:                  fieldGuessEnum(commodityTypes),
		FieldNameOriginalPrice:         fieldGuessNumber,
		FieldNameOriginalPriceCurrency: fieldGuessString,
		FieldNameSerialNumber:          fieldGuessString,
		FieldNameURLs:                  fieldGuessStringArray,
		FieldNamePurchaseDate:          fieldGuessString,
		FieldNameWarrantyExpiresAt:     fieldGuessString,
		FieldNameComments:              fieldGuessString,
	}

	warningSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"code": map[string]any{
				"type": "string",
				"enum": []string{
					"low_confidence",
					"unreadable_serial",
					"ambiguous_price",
					"currency_inferred",
					"no_photo_text",
					"multiple_items",
				},
			},
			"field":  map[string]any{"type": "string"},
			"detail": map[string]any{"type": "string"},
		},
		"required":             []string{"code"},
		"additionalProperties": false,
	}

	fieldsObject := map[string]any{
		"type":                 "object",
		"properties":           fields,
		"additionalProperties": false,
	}

	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"fields": fieldsObject,
			// items is present ONLY when the source describes more than one
			// distinct product; each entry mirrors the `fields` shape so the
			// FE can render/accept a chosen item with the same machinery.
			"items": map[string]any{
				"type":        "array",
				"description": "Present ONLY when the source (e.g. a multi-line receipt or invoice) contains more than one distinct purchased product. One entry per product, most prominent first; exclude tax/subtotal/shipping/discount/total lines.",
				"items": map[string]any{
					"type":                 "object",
					"properties":           map[string]any{"fields": fieldsObject},
					"required":             []string{"fields"},
					"additionalProperties": false,
				},
			},
			"warnings": map[string]any{
				"type":  "array",
				"items": warningSchema,
			},
		},
		"required":             []string{"fields"},
		"additionalProperties": false,
	}
}

// ResponseSchemaJSON returns the schema serialised as JSON bytes, which
// is the form OpenAI's response_format and Anthropic's tool input_schema
// expect.
func ResponseSchemaJSON() ([]byte, error) {
	return json.Marshal(ResponseSchema())
}

// rawScanResponse mirrors the JSON shape promised by the schema. Both
// providers unmarshal upstream JSON into this struct, then ToScanResult
// converts to the public ScanResult type.
type rawScanResponse struct {
	Fields   map[string]rawFieldGuess `json:"fields"`
	Items    []rawScanItem            `json:"items"`
	Warnings []Warning                `json:"warnings"`
}

// rawScanItem mirrors one entry of the optional multi-item array.
type rawScanItem struct {
	Fields map[string]rawFieldGuess `json:"fields"`
}

// rawFieldGuess uses json.RawMessage on Value so the same schema can
// accommodate string / number / []string at the wire level — the
// concrete Go value is decided at conversion time based on the field
// name. Vendors that respect the schema send a typed value; vendors
// that don't (or buggy responses) still parse without exploding.
type rawFieldGuess struct {
	Value      json.RawMessage `json:"value"`
	Confidence float64         `json:"confidence"`
}

// ToScanResult parses the JSON returned by the upstream call into the
// public ScanResult shape. Unknown field names are dropped (defence in
// depth against schema drift); per-field type mismatches are demoted to
// a "low_confidence" warning so the response is still usable.
func ToScanResult(body []byte) (*ScanResult, error) {
	var raw rawScanResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse vision response: %w", err)
	}

	out := &ScanResult{
		Warnings: append([]Warning(nil), raw.Warnings...),
	}

	fields, fieldWarnings := decodeFieldMap(raw.Fields)
	out.Fields = fields
	out.Warnings = append(out.Warnings, fieldWarnings...)

	// Optional multi-item list. Per-item type mismatches are dropped
	// silently (the primary fields already surfaced any warning); empty
	// items are skipped so a stray {} doesn't render a blank choice.
	for _, it := range raw.Items {
		itemFields, _ := decodeFieldMap(it.Fields)
		if len(itemFields) == 0 {
			continue
		}
		out.Items = append(out.Items, ScanItem{Fields: itemFields})
	}

	// Defensive: if the model only populated items and left fields empty,
	// mirror the most prominent item so single-item consumers (and the
	// audit ResultJSON) still see a primary extraction.
	if len(out.Fields) == 0 && len(out.Items) > 0 {
		out.Fields = out.Items[0].Fields
	}

	return out, nil
}

// decodeFieldMap converts a raw vendor field map into the public FieldGuess
// map, dropping unknown keys and type-mismatched values. It returns the
// decoded fields plus a (possibly empty) list of "low_confidence" warnings
// for values whose type didn't match the field's expected shape. Shared by
// the primary `fields` and each `items[].fields`.
func decodeFieldMap(raw map[string]rawFieldGuess) (map[string]FieldGuess, []Warning) {
	out := make(map[string]FieldGuess, len(raw))
	var warnings []Warning
	for _, name := range AllFieldNames {
		rg, ok := raw[name]
		if !ok {
			continue
		}
		val, ok := decodeFieldValue(name, rg.Value)
		if !ok {
			warnings = append(warnings, Warning{
				Code:   "low_confidence",
				Field:  name,
				Detail: "field value did not match expected type",
			})
			continue
		}
		out[name] = FieldGuess{Value: val, Confidence: rg.Confidence}
	}
	return out, warnings
}

// decodeFieldValue maps the JSON-typed raw value to the Go concrete
// type expected for that field. Returns (value, true) on success and
// (nil, false) on any type mismatch.
func decodeFieldValue(name string, raw json.RawMessage) (any, bool) {
	switch name {
	case FieldNameOriginalPrice:
		var n float64
		if err := json.Unmarshal(raw, &n); err != nil {
			return nil, false
		}
		return n, true
	case FieldNameURLs:
		var s []string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, false
		}
		return s, true
	default:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, false
		}
		return s, true
	}
}
