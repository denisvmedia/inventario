package aivision

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// SystemPrompt is the role/system instruction sent to every vendor. It
// is intentionally short — vendors trim very long system prompts and a
// concise instruction reduces token spend on every call.
const SystemPrompt = "You are an assistant that extracts structured product information from photos and documents of physical items.\n" +
	"You will receive 1 to 5 inputs — photos and/or PDFs. They may show a product (front, back, label, packaging, etc.) " +
	"or a purchase document such as a receipt, invoice, or product manual. A receipt/invoice may itself be a PDF, a scan, " +
	"an export, or a photo (jpg/png/heic) — handle it the same regardless of format.\n" +
	"Return \"items\": a JSON array with ONE entry per distinct purchased product, most prominent / most expensive first. " +
	"Each entry is that product's field object directly (name, short_name, type, original_price, original_price_currency, etc.).\n" +
	"A single product → a one-element array. A receipt or invoice that lists SEVERAL products → one entry for EACH " +
	"product line. Do NOT collapse them into one and do NOT pick just one — enumerate EVERY product on the document.\n" +
	"EXCLUDE non-product lines: subtotals, taxes/VAT, shipping/postage, discounts, vouchers, fees, deposits, and totals.\n" +
	"For each product, from a receipt/invoice read its own purchase price, currency, and purchase date; " +
	"the price is the amount of that product line, NEVER the receipt total. " +
	"When a line covers several identical units, set \"quantity\" to the number of units and keep the whole line amount in \"original_price\"; " +
	"put the seller/vendor/store name into \"seller\", not into \"comments\".\n" +
	"Classify \"type\" as EXACTLY one of the allowed values in the schema enum; omit it if none clearly fits.\n" +
	"Keep \"short_name\" a concise label of at most 40 characters.\n" +
	"Put the manufacturer's name into \"brand\" and the model name or number, as printed on the product, label, or document, into \"model\"; " +
	"do not repeat them in \"name\" or \"short_name\" unless they are part of the product's usual name.\n" +
	"Suggest 2–5 short, lowercase \"tags\" describing each product (category, material, colour).\n" +
	"If a warranty period or expiry is shown, set \"warranty_expires_at\" to the warranty END date (YYYY-MM-DD); " +
	"if only a duration like \"2 years\" is given and the purchase date is known, add the duration to the purchase date.\n" +
	"Each attached file is preceded by a text label \"File N\": N is the file's index, counted from 0 in the order the files are attached.\n" +
	"Also return \"files\": ONE entry per attached file with its \"index\", its \"document_type\" and a \"title\". " +
	"\"document_type\" is invoice, receipt, manual, warranty, photo (a picture of the product itself) or other. " +
	"\"title\" is a short readable title for the file, in the language of the document, at most 120 characters, " +
	"such as \"Bosch invoice, March 2024\".\n" +
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
	if n := len(req.Photos); n == 1 {
		b.WriteString("There is 1 attached file, labeled File 0. Return one \"files\" entry for it.\n")
	} else if n > 1 {
		fmt.Fprintf(&b, "There are %d attached files, labeled File 0 to File %d. Return one \"files\" entry for each.\n", n, n-1)
	}
	if req.PreferredCurrencyCode != "" {
		fmt.Fprintf(&b, "If a price is visible without a clear currency symbol, prefer currency code %q.\n", req.PreferredCurrencyCode)
	}
	if req.HintFromUser != "" {
		fmt.Fprintf(&b, "User hint: %s\n", req.HintFromUser)
	}
	b.WriteString("Allowed values for \"type\", each followed by the name the user knows it by:\n")
	for _, t := range req.itemTypes() {
		if t.Name == "" || t.Name == t.Slug {
			fmt.Fprintf(&b, "- %s\n", t.Slug)
			continue
		}
		fmt.Fprintf(&b, "- %s (%s)\n", t.Slug, t.Name)
	}
	if fields := req.scanCustomFields(); len(fields) > 0 {
		b.WriteString("The user also records these custom fields. Put a value into \"custom_fields\" under the field's key " +
			"only when the product or document shows it; dates as YYYY-MM-DD, a select value as one of its options:\n")
		for _, f := range fields {
			fmt.Fprintf(&b, "- %s: %q (%s", f.Key, f.Name, f.Type)
			if len(f.Options) > 0 {
				fmt.Fprintf(&b, "; options: %s", strings.Join(quoteAll(f.Options), ", "))
			}
			b.WriteString(")\n")
		}
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
// optional so the model is free to omit anything it can't read. The values
// allowed for "type" are the slugs of req's item types.
func ResponseSchema(req ScanRequest) map[string]any {
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
	// truncates defensively). Used for short_name (40 chars) and for the
	// brand, the model and the seller (100 each).
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

	// commodityTypes are the slugs of the group's item types. The FE checks
	// a guess against the same list before it pre-fills the form, so a guess
	// outside it only costs a dropped type, never a bad write.
	options := req.itemTypes()
	commodityTypes := make([]string, 0, len(options))
	for _, t := range options {
		commodityTypes = append(commodityTypes, t.Slug)
	}

	fields := map[string]any{
		FieldNameName:                  fieldGuessString,
		FieldNameShortName:             fieldGuessStringMax(40),
		FieldNameBrand:                 fieldGuessStringMax(100),
		FieldNameModel:                 fieldGuessStringMax(100),
		FieldNameSeller:                fieldGuessStringMax(100),
		FieldNameType:                  fieldGuessEnum(commodityTypes),
		FieldNameOriginalPrice:         fieldGuessNumber,
		FieldNameOriginalPriceCurrency: fieldGuessString,
		FieldNameSerialNumber:          fieldGuessString,
		FieldNameURLs:                  fieldGuessStringArray,
		FieldNamePurchaseDate:          fieldGuessString,
		FieldNameWarrantyExpiresAt:     fieldGuessString,
		FieldNameComments:              fieldGuessString,
		FieldNameTags:                  fieldGuessStringArray,
		FieldNameDocumentType:          fieldGuessEnum(ScanDocumentTypes),
		FieldNameQuantity:              fieldGuessNumber,
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

	if custom := req.scanCustomFields(); len(custom) > 0 {
		properties := make(map[string]any, len(custom))
		for _, f := range custom {
			properties[f.Key] = customFieldValueSchema(f)
		}
		fields[FieldNameCustomFields] = map[string]any{
			"type":                 "object",
			"properties":           properties,
			"additionalProperties": false,
		}
	}

	fieldsObject := map[string]any{
		"type":                 "object",
		"properties":           fields,
		"additionalProperties": false,
	}

	// items is the REQUIRED primary output: a list of products. Making the
	// model produce a list (rather than a single `fields` object) is what
	// reliably gets every line of a multi-product invoice enumerated instead
	// of the model picking just one. A single product is simply a 1-element
	// array. The service derives the single-item `fields` from items[0].
	// files is required too: one suggestion per attached file. The
	// properties are ResponseProperties so the products come first.
	return map[string]any{
		"type": "object",
		"properties": ResponseProperties{
			FieldNameFiles: filesSchema(len(req.Photos), fieldGuessEnum(FileDocumentTypes), fieldGuessStringMax(MaxFileTitleRunes)),
			"items": map[string]any{
				"type":     "array",
				"minItems": 1,
				"description": "ONE entry per distinct purchased product (most prominent / most expensive first). " +
					"A single product is a one-element array; a multi-line receipt or invoice has one entry for EACH " +
					"product line — never collapse them and never return just one. " +
					"Exclude tax/subtotal/shipping/discount/total lines. Each entry is the product's field object.",
				// Each item IS the field object directly (flat) — one nesting
				// level, which the model follows far more reliably than a
				// wrapper object. ToScanResult also accepts the legacy
				// {"fields": {...}} shape defensively.
				"items": fieldsObject,
			},
			"warnings": map[string]any{
				"type":  "array",
				"items": warningSchema,
			},
		},
		"required":             []string{"items", FieldNameFiles},
		"additionalProperties": false,
	}
}

// ResponseProperties are the members of the response object. They marshal
// in the order the model should write them: the products, then the file
// suggestions, then the warnings, and any other member after those in name
// order. A model writes its answer in the order of the schema, so the
// product lines of a long receipt come before the file suggestions in the
// output budget. A plain map would marshal "files" first.
type ResponseProperties map[string]any

// responsePropertyOrder ranks the members of ResponseProperties.
var responsePropertyOrder = []string{"items", FieldNameFiles, "warnings"}

// MarshalJSON writes the members in the order of responsePropertyOrder.
func (p ResponseProperties) MarshalJSON() ([]byte, error) {
	rank := func(name string) int {
		if i := slices.Index(responsePropertyOrder, name); i >= 0 {
			return i
		}
		return len(responsePropertyOrder)
	}
	names := slices.Collect(maps.Keys(p))
	slices.SortFunc(names, func(a, b string) int {
		return cmp.Or(cmp.Compare(rank(a), rank(b)), strings.Compare(a, b))
	})
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, name := range names {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(name)
		if err != nil {
			return nil, err
		}
		value, err := json.Marshal(p[name])
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(value)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// filesSchema is the schema of the per-file suggestions: one entry per
// attached file, each naming its file by index. When the request's file
// count is known it bounds both the list and the index.
func filesSchema(fileCount int, documentType, title map[string]any) map[string]any {
	index := map[string]any{"type": "integer", "minimum": 0}
	list := map[string]any{
		"type": "array",
		"description": "ONE entry per attached file. index is the N of the file's \"File N\" label, counted from 0 " +
			"in the order the files are attached.",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"index":         index,
				"document_type": documentType,
				"title":         title,
			},
			"required":             []string{"index", "document_type", "title"},
			"additionalProperties": false,
		},
	}
	if fileCount > 0 {
		index["maximum"] = fileCount - 1
		list["minItems"] = fileCount
		list["maxItems"] = fileCount
	}
	return list
}

// FieldNameCustomFields is the key of the custom field values in a response
// item. It is not a FieldName: its value is an object of guesses keyed by
// custom field key, see ScanResult.CustomFields.
const FieldNameCustomFields = "custom_fields"

// FieldNameFiles is the key of the per-file suggestions in a response, see
// ScanResult.Files. A warning about a dropped file suggestion names it as
// its field.
const FieldNameFiles = "files"

// customFieldValueSchema returns the guess schema of one custom field, or nil
// for a type the model cannot fill.
func customFieldValueSchema(f CustomFieldOption) map[string]any {
	var value map[string]any
	switch f.Type {
	case "text", "long_text", "url":
		value = map[string]any{"type": "string"}
	case "date":
		value = map[string]any{"type": "string", "description": "YYYY-MM-DD"}
	case "number":
		value = map[string]any{"type": "number"}
	case "boolean":
		value = map[string]any{"type": "boolean"}
	case "select":
		if len(f.Options) == 0 {
			return nil
		}
		value = map[string]any{"type": "string", "enum": f.Options}
	default:
		return nil
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"value":      value,
			"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		},
		"required":             []string{"value", "confidence"},
		"additionalProperties": false,
	}
}

func quoteAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = fmt.Sprintf("%q", s)
	}
	return out
}

// ResponseSchemaJSON returns the schema serialised as JSON bytes, which
// is the form OpenAI's response_format and Anthropic's tool input_schema
// expect.
func ResponseSchemaJSON(req ScanRequest) ([]byte, error) {
	return json.Marshal(ResponseSchema(req))
}

// rawScanResponse mirrors the JSON shape promised by the schema. Both
// providers unmarshal upstream JSON into this struct, then ToScanResult
// converts to the public ScanResult type.
type rawScanResponse struct {
	Fields map[string]rawFieldGuess `json:"fields"`
	Items  []json.RawMessage        `json:"items"`
	// Files stays raw so a malformed list costs the file suggestions only,
	// never the scan.
	Files    json.RawMessage `json:"files"`
	Warnings []Warning       `json:"warnings"`
}

// decodeRawItem decodes one entry of the items[] array into a field map. It
// accepts BOTH shapes so model variance can't blank the result:
//   - flat: the entry IS the field map (the schema we now request),
//     e.g. {"name": {...}, "type": {...}}
//   - nested: a {"fields": {...}} wrapper (legacy / some responses)
//
// The flat shape is tried first; when it holds no canonical field, the
// wrapper is read. The custom field values sit under FieldNameCustomFields,
// next to the wrapper or inside it; the ones next to it win.
func decodeRawItem(raw json.RawMessage) ScanItem {
	var flat map[string]json.RawMessage
	if json.Unmarshal(raw, &flat) != nil {
		return ScanItem{}
	}
	item := ScanItem{Fields: decodeFlatFields(flat), CustomFields: decodeCustomFields(flat[FieldNameCustomFields])}
	if len(item.Fields) > 0 {
		return item
	}
	var fields map[string]json.RawMessage
	if nested, ok := flat["fields"]; ok && json.Unmarshal(nested, &fields) == nil {
		item.Fields = decodeFlatFields(fields)
		if item.CustomFields == nil {
			item.CustomFields = decodeCustomFields(fields[FieldNameCustomFields])
		}
	}
	return item
}

// decodeFlatFields decodes the canonical fields of one item, skipping a key
// whose value is not a guess object.
func decodeFlatFields(raw map[string]json.RawMessage) map[string]FieldGuess {
	guesses := make(map[string]rawFieldGuess, len(raw))
	for name, v := range raw {
		var g rawFieldGuess
		if json.Unmarshal(v, &g) == nil {
			guesses[name] = g
		}
	}
	f, _ := decodeFieldMap(guesses)
	if len(f) == 0 {
		return nil
	}
	return f
}

// decodeCustomFields decodes the custom field guesses of one item. A value
// that is not a string, a number or a boolean is dropped; the service checks
// the rest against the fields.
func decodeCustomFields(raw json.RawMessage) map[string]FieldGuess {
	if len(raw) == 0 {
		return nil
	}
	var guesses map[string]rawFieldGuess
	if json.Unmarshal(raw, &guesses) != nil {
		return nil
	}
	out := make(map[string]FieldGuess, len(guesses))
	for key, g := range guesses {
		if len(g.Value) == 0 || string(g.Value) == "null" {
			continue
		}
		var (
			s string
			n float64
			b bool
		)
		switch {
		case json.Unmarshal(g.Value, &s) == nil:
			out[key] = FieldGuess{Value: s, Confidence: g.Confidence}
		case json.Unmarshal(g.Value, &n) == nil:
			out[key] = FieldGuess{Value: n, Confidence: g.Confidence}
		case json.Unmarshal(g.Value, &b) == nil:
			out[key] = FieldGuess{Value: b, Confidence: g.Confidence}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
// public ScanResult shape. fileCount is the number of files the request
// carried; see decodeFileGuesses for how the file suggestions are checked
// against it. Unknown field names are dropped (defence in depth against
// schema drift); per-field type mismatches are demoted to a
// "low_confidence" warning so the response is still usable.
func ToScanResult(body []byte, fileCount int) (*ScanResult, error) {
	var raw rawScanResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse vision response: %w", err)
	}

	out := &ScanResult{
		Warnings: append([]Warning(nil), raw.Warnings...),
	}
	var fileWarnings []Warning
	out.Files, fileWarnings = decodeFileGuesses(raw.Files, fileCount)
	out.Warnings = append(out.Warnings, fileWarnings...)

	// items is the model's primary output now (one entry per product). Decode
	// each, dropping empties so a stray {} never renders a blank choice.
	var products []ScanItem
	for _, it := range raw.Items {
		item := decodeRawItem(it)
		if len(item.Fields) == 0 && len(item.CustomFields) == 0 {
			continue
		}
		products = append(products, item)
	}

	// Primary single-item view: prefer an explicit top-level `fields`
	// (back-compat with older single-product responses and provider tests),
	// otherwise the first product. Type-mismatch warnings surface for the
	// primary only.
	primary, primaryWarnings := decodeFieldMap(raw.Fields)
	switch {
	case len(primary) > 0:
		out.Fields = primary
		out.Warnings = append(out.Warnings, primaryWarnings...)
	case len(products) > 0:
		out.Fields = products[0].Fields
		out.CustomFields = products[0].CustomFields
	}

	// Expose Items to the FE only when there's an actual choice to make.
	if len(products) > 1 {
		out.Items = products
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
	case FieldNameOriginalPrice, FieldNameQuantity:
		var n float64
		if err := json.Unmarshal(raw, &n); err != nil {
			return nil, false
		}
		return n, true
	case FieldNameURLs, FieldNameTags:
		var s []string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, false
		}
		return s, true
	case FieldNameDocumentType:
		return decodeEnum(raw, ScanDocumentTypes)
	default:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, false
		}
		return s, true
	}
}
