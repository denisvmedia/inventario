package aivision

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SystemPrompt is the role/system instruction sent to every vendor. It
// is intentionally short — vendors trim very long system prompts and a
// concise instruction reduces token spend on every call.
const SystemPrompt = `You are an assistant that extracts structured product information from photos of physical items.
You will receive 1 to 5 photos of a single product (front, back, label, packaging, etc.).
Return ONE JSON object that matches the requested schema EXACTLY. Do not include any prose, markdown, or extra keys.
For each field, ALWAYS include a "confidence" score between 0.0 and 1.0 reflecting how sure you are.
Omit fields you have NO evidence for rather than guessing — null/empty is preferred over hallucinated values.`

// UserPromptHeader is the literal text prepended to the multimodal user
// turn. It tells the model what task to perform; the actual schema is
// delivered via the per-vendor structured output channel (Anthropic
// tool-use schema, OpenAI response_format json_schema).
func UserPromptHeader(req ScanRequest) string {
	var b strings.Builder
	b.WriteString("Extract the product information from the attached photo(s).\n")
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

	fields := map[string]any{
		FieldNameName:                  fieldGuessString,
		FieldNameShortName:             fieldGuessString,
		FieldNameType:                  fieldGuessString,
		FieldNameOriginalPrice:         fieldGuessNumber,
		FieldNameOriginalPriceCurrency: fieldGuessString,
		FieldNameSerialNumber:          fieldGuessString,
		FieldNameURLs:                  fieldGuessStringArray,
		FieldNamePurchaseDate:          fieldGuessString,
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
				},
			},
			"field":  map[string]any{"type": "string"},
			"detail": map[string]any{"type": "string"},
		},
		"required":             []string{"code"},
		"additionalProperties": false,
	}

	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"fields": map[string]any{
				"type":                 "object",
				"properties":           fields,
				"additionalProperties": false,
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
	Warnings []Warning                `json:"warnings"`
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
		Fields:   make(map[string]FieldGuess, len(raw.Fields)),
		Warnings: append([]Warning(nil), raw.Warnings...),
	}

	for _, name := range AllFieldNames {
		rg, ok := raw.Fields[name]
		if !ok {
			continue
		}
		val, ok := decodeFieldValue(name, rg.Value)
		if !ok {
			// Field came back but in the wrong shape. Surface as a
			// warning so the FE can still render the rest of the
			// extraction; drop the unparseable value.
			out.Warnings = append(out.Warnings, Warning{
				Code:   "low_confidence",
				Field:  name,
				Detail: "field value did not match expected type",
			})
			continue
		}
		out.Fields[name] = FieldGuess{Value: val, Confidence: rg.Confidence}
	}

	return out, nil
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
