// Package aivision implements the AI vision / photo-scan abstraction used
// by the Add Item dialog (issue #1720) to extract structured commodity
// fields from one or more product photos.
//
// The package is deliberately vendor-neutral: callers depend only on the
// Provider interface and the structured ScanRequest/ScanResult types, and
// pick a concrete implementation at boot via NewProvider(cfg). The HTTP
// client is carried on ProviderConfig.HTTPClient so a shared client (or
// a test RoundTripper) can be threaded in without changing the
// constructor signature. Two real providers ship in the tree
// (Anthropic Claude vision, OpenAI GPT-4o vision) plus a deterministic
// Mock used by unit tests and e2e runs that need a stable canned result
// without burning credits or requiring network access.
//
// The interface intentionally exposes a single Scan call rather than a
// streaming or chunked surface — the FE issues one HTTP request with all
// photos as a multipart body, waits, and renders the structured result.
// The handler enforces server-side limits (photo count, per-photo bytes,
// rate limit per user) before the provider is ever called.
package aivision

import (
	"context"

	"github.com/go-extras/errx"
)

// Provider is the abstract vision/scan dependency consumed by
// CommodityScanService. Implementations wrap a specific vendor API
// (Anthropic, OpenAI) or a canned deterministic source (Mock).
//
// Implementations MUST NOT log API keys or any sensitive header value.
// All errors should classify (via errxtrace.Classify) one of the
// sentinels declared in this package so the service layer can map them
// to stable HTTP statuses without string-matching upstream messages.
type Provider interface {
	// Name returns a stable identifier for the provider, used in the
	// audit table (provider, model). Examples: "anthropic", "openai",
	// "mock".
	Name() string

	// Model returns the specific upstream model identifier the provider
	// is configured to call (e.g. "claude-sonnet-4-6", "gpt-4o"). Used
	// for the audit row so the cost / accuracy dashboards can compare
	// across deployments. May be empty when a provider has no notion of
	// a model (e.g. the in-process mock).
	Model() string

	// Scan accepts a ScanRequest and returns the structured ScanResult.
	// Implementations are expected to honour ctx cancellation/deadline
	// and to surface upstream auth/quota/timeout errors as one of the
	// declared sentinels (ErrProviderUnavailable / ErrProviderTimeout /
	// ErrProviderAuth). Any non-classified error falls through to the
	// generic provider-error path in the calling service.
	Scan(ctx context.Context, req ScanRequest) (*ScanResult, error)
}

// PhotoInput is a single user-uploaded image handed to the provider as
// raw bytes plus its (already-detected) MIME type. The service guarantees
// MIME and size pre-checks; providers may still reject content the
// upstream vendor declines (e.g. some vendors reject HEIC).
type PhotoInput struct {
	// Filename is the original filename for diagnostics; never used as
	// a security boundary.
	Filename string
	// ContentType is the detected MIME type (image/jpeg, image/png,
	// image/webp, image/heic, image/heif).
	ContentType string
	// Data is the raw image bytes.
	Data []byte
}

// ScanRequest is the structured input to Provider.Scan. All fields apart
// from Photos are optional hints that improve prompt quality without
// changing the response shape.
type ScanRequest struct {
	// Photos is the ordered list of images to analyse. Length is at
	// least 1; the caller enforces the upper bound.
	Photos []PhotoInput
	// HintFromUser is an optional free-form user hint (brand, category
	// guess, model number visible elsewhere). Currently always empty;
	// reserved for a future affordance on the Add Item dialog.
	HintFromUser string
	// PreferredCurrencyCode is the tenant's main currency (e.g. "USD",
	// "EUR"). The prompt asks the model to prefer this code when a
	// price is visible but the currency symbol is ambiguous.
	PreferredCurrencyCode string
}

// FieldGuess is a single extracted field value plus a 0..1 confidence
// score. Confidence is provider-reported, not calibrated — the FE uses
// it only for the per-field "review confidence" badge, never for hard
// gating. Value's concrete type depends on the field:
//
//   - name, short_name, type, serial_number, comments,
//     original_price_currency: string
//   - original_price: float64 (decimal as string is also accepted by
//     callers, which coerce it to a number)
//   - urls: []string
//   - purchase_date: string in YYYY-MM-DD form
type FieldGuess struct {
	Value      any     `json:"value"`
	Confidence float64 `json:"confidence"`
}

// Warning is a non-fatal note attached to a ScanResult — e.g. "I think
// this is a serial number but I can't read every character", "I inferred
// currency from the dominant text language". Surfaces in the FE banner
// so the user knows which fields to double-check.
type Warning struct {
	// Code is a stable identifier the FE branches on. Known values:
	// "low_confidence", "unreadable_serial", "ambiguous_price",
	// "currency_inferred", "no_photo_text".
	Code string `json:"code"`
	// Field is the affected ScanResult.Fields key, or "" when the
	// warning is global.
	Field string `json:"field,omitempty"`
	// Detail is a short human-readable explanation. Localisation is
	// the FE's responsibility — the provider returns English here.
	Detail string `json:"detail,omitempty"`
}

// ScanResult is the structured output of Provider.Scan. Fields is a map
// keyed by canonical field name (see the FieldGuess doc-comment for the
// closed set); Warnings is the (possibly empty) list of non-fatal notes;
// UsedTokens and LatencyMS are server-measured metrics that feed the
// audit table.
type ScanResult struct {
	// Fields is keyed by canonical field name. The provider populates
	// only fields it has evidence for; absent keys mean "no signal" and
	// the FE leaves the form input blank.
	Fields map[string]FieldGuess `json:"fields"`
	// Warnings is the non-fatal note list. nil and empty are
	// equivalent.
	Warnings []Warning `json:"warnings,omitempty"`
	// UsedTokens is the provider-reported token usage (sum of input +
	// output where the upstream API distinguishes them). Zero when
	// unavailable.
	UsedTokens int `json:"used_tokens,omitempty"`
	// LatencyMS is the wall-clock duration of the upstream call,
	// measured server-side, used for audit and observability.
	LatencyMS int64 `json:"latency_ms,omitempty"`
}

// Sentinel errors returned by Provider implementations and the registry
// constructor. Callers (CommodityScanService) classify these into HTTP
// status codes in their own sentinel set so the apiserver error mapping
// keeps the provider type out of the HTTP layer.
var (
	// ErrProviderDisabled is returned by NewProvider when the
	// configuration selects "none" — the service is intentionally not
	// wired up. The handler maps this to 503.
	ErrProviderDisabled = errx.NewSentinel("aivision provider is disabled in configuration")

	// ErrProviderUnknown is returned by NewProvider when the
	// configuration names a provider that no implementation is
	// registered for. Distinct from ErrProviderDisabled so operators
	// can tell "typo in config" from "this deployment intentionally
	// turned it off".
	ErrProviderUnknown = errx.NewSentinel("aivision provider name is not recognised")

	// ErrProviderUnavailable is returned by a Provider when the
	// upstream API was reachable but rejected the request in a way
	// that's the user's fault to retry (rate limit at the vendor,
	// service degraded, etc). Maps to 502.
	ErrProviderUnavailable = errx.NewSentinel("aivision provider is currently unavailable")

	// ErrProviderTimeout is returned when the context deadline fires
	// before the upstream responded. Maps to 504.
	ErrProviderTimeout = errx.NewSentinel("aivision provider timed out")

	// ErrProviderAuth is returned when the upstream rejected the
	// configured API key (401/403). Always a 500 to the client — the
	// FE has no recourse and we don't want to leak credential state.
	ErrProviderAuth = errx.NewSentinel("aivision provider authentication failed")

	// ErrProviderBadResponse is returned when the upstream returned a
	// success status but the body could not be parsed as the agreed
	// structured shape. Maps to 502.
	ErrProviderBadResponse = errx.NewSentinel("aivision provider returned an unparseable response")
)

// FieldName is a typed alias for the canonical extraction keys. Using a
// named type makes grep across the FE/BE for "where do we read field X"
// trivially correct.
type FieldName = string

// The closed set of canonical field names the FE knows how to render in
// the Add Item dialog. New fields require a coordinated FE change, so
// keeping the set in one place catches typos at compile time when the
// service marshals.
const (
	FieldNameName                  FieldName = "name"
	FieldNameShortName             FieldName = "short_name"
	FieldNameType                  FieldName = "type"
	FieldNameOriginalPrice         FieldName = "original_price"
	FieldNameOriginalPriceCurrency FieldName = "original_price_currency"
	FieldNameSerialNumber          FieldName = "serial_number"
	FieldNameURLs                  FieldName = "urls"
	FieldNamePurchaseDate          FieldName = "purchase_date"
	FieldNameComments              FieldName = "comments"
)

// AllFieldNames is the closed set used by tests and by the prompt
// builder to enumerate the expected response keys.
var AllFieldNames = []FieldName{
	FieldNameName,
	FieldNameShortName,
	FieldNameType,
	FieldNameOriginalPrice,
	FieldNameOriginalPriceCurrency,
	FieldNameSerialNumber,
	FieldNameURLs,
	FieldNamePurchaseDate,
	FieldNameComments,
}
