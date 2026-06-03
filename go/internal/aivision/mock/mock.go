// Package mock provides a deterministic, network-free Provider used by
// unit tests and e2e runs. The default canned ScanResult exercises every
// field the FE renders so screenshot/visual review always has something
// to display; callers that need a specific shape attach an override via
// WithResultOverride(ctx, result).
package mock

import (
	"context"
	"strings"
	"time"

	"github.com/denisvmedia/inventario/internal/aivision"
)

// Name is the stable identifier this provider reports for the audit
// table and configuration discriminator.
const Name = "mock"

// init wires the mock provider into the aivision registry so callers
// can select it by name via aivision.NewProvider. Standard registry-
// pattern init; side-effect imports in the bootstrap layer bring this
// runtime registration into the binary.
//
//nolint:gochecknoinits // standard registry-pattern provider registration
func init() {
	aivision.RegisterProvider(Name, func(_ aivision.ProviderConfig) (aivision.Provider, error) {
		return New(), nil
	})
}

// Provider is the deterministic implementation of aivision.Provider.
// Construct with New(); the zero value also works but the configurable
// fields are exported only via the constructor to keep the test API
// terse.
type Provider struct {
	defaultResult aivision.ScanResult
	defaultErr    error
}

// New creates a Provider with the default canned ScanResult. Callers
// can swap the canned default by passing options, or override per-call
// via WithResultOverride on the request context.
func New(opts ...Option) *Provider {
	p := &Provider{
		defaultResult: DefaultResult(),
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Option mutates a Provider during construction.
type Option func(*Provider)

// WithDefaultResult overrides the canned result returned when the
// context carries no per-call override.
func WithDefaultResult(r aivision.ScanResult) Option {
	return func(p *Provider) { p.defaultResult = r }
}

// WithDefaultError makes every Scan return the given error (unless the
// context carries an override). Useful for asserting the service's
// error-handling paths from a single shared fixture.
func WithDefaultError(err error) Option {
	return func(p *Provider) { p.defaultErr = err }
}

// Name implements aivision.Provider.
func (*Provider) Name() string { return Name }

// Model implements aivision.Provider. The mock provider does not call
// a real upstream model; the constant identifier makes audit rows
// originating from tests immediately recognisable.
func (*Provider) Model() string { return "mock" }

// Scan implements aivision.Provider. It returns the per-call override
// from ctx when present, otherwise the constructor's defaults.
//
// The returned ScanResult is a deep copy of the source (override or
// default): a shallow struct copy still shares the Fields map and the
// Warnings slice, so a caller that mutates either would contaminate the
// next call's result. Deep-copying keeps the mock's behaviour identical
// to a real provider (which always allocates a fresh struct off the
// wire) and avoids action-at-a-distance test failures.
func (p *Provider) Scan(ctx context.Context, req aivision.ScanRequest) (*aivision.ScanResult, error) {
	if override, ok := overrideFromContext(ctx); ok {
		if override.err != nil {
			return nil, override.err
		}
		result := cloneScanResult(override.result)
		return &result, nil
	}
	if p.defaultErr != nil {
		return nil, p.defaultErr
	}
	// Simulate a multi-line invoice when the filename looks like one, so the
	// item-chooser flow is exercisable end-to-end with the deterministic
	// mock provider (preview stacks / e2e) without a real vendor. We key off
	// the FILENAME, not the MIME type: an invoice may be a PDF, a scan, an
	// export, or a photo (jpg/png/heic). Anything else returns the
	// single-item default.
	if looksLikeInvoice(req.Photos) {
		result := cloneScanResult(MultiItemResult())
		return &result, nil
	}
	result := cloneScanResult(p.defaultResult)
	return &result, nil
}

// looksLikeInvoice is the mock's filename heuristic for "this is a multi-line
// receipt/invoice" — format-agnostic on purpose (an invoice can be a PDF, a
// scan, an export, or a photo). Name a file invoice.* / receipt.* / bill.* /
// faktura.* / paragon.* / uctenka.* to demo the chooser. Real providers
// decide from the actual content, not the name.
func looksLikeInvoice(photos []aivision.PhotoInput) bool {
	keywords := []string{"invoice", "receipt", "bill", "faktura", "paragon", "uctenka", "účtenka"}
	for _, ph := range photos {
		name := strings.ToLower(ph.Filename)
		for _, kw := range keywords {
			if strings.Contains(name, kw) {
				return true
			}
		}
	}
	return false
}

// cloneScanResult returns a deep copy of src. Fields/Items (maps + slice)
// are reallocated; FieldGuess.Value is cloned per cloneFieldValue. Deep-
// copying keeps the mock identical to a real provider (fresh struct off
// the wire) and avoids action-at-a-distance across calls.
func cloneScanResult(src aivision.ScanResult) aivision.ScanResult {
	dst := aivision.ScanResult{
		UsedTokens: src.UsedTokens,
		LatencyMS:  src.LatencyMS,
		Fields:     cloneFieldMap(src.Fields),
	}
	if src.Items != nil {
		dst.Items = make([]aivision.ScanItem, len(src.Items))
		for i, it := range src.Items {
			dst.Items[i] = aivision.ScanItem{Fields: cloneFieldMap(it.Fields)}
		}
	}
	if src.Warnings != nil {
		dst.Warnings = make([]aivision.Warning, len(src.Warnings))
		copy(dst.Warnings, src.Warnings)
	}
	return dst
}

// cloneFieldMap deep-copies a canonical field map (nil stays nil). Shared
// by the primary Fields and each item's Fields.
func cloneFieldMap(src map[string]aivision.FieldGuess) map[string]aivision.FieldGuess {
	if src == nil {
		return nil
	}
	dst := make(map[string]aivision.FieldGuess, len(src))
	for k, v := range src {
		dst[k] = aivision.FieldGuess{Value: cloneFieldValue(v.Value), Confidence: v.Confidence}
	}
	return dst
}

// cloneFieldValue defensively copies the slice-typed values we know about
// (the []string fields: urls and tags). Scalar types pass through unchanged.
func cloneFieldValue(v any) any {
	if v == nil {
		return nil
	}
	if s, ok := v.([]string); ok {
		out := make([]string, len(s))
		copy(out, s)
		return out
	}
	return v
}

// override is the context-stored per-call result/error pair.
type override struct {
	result aivision.ScanResult
	err    error
}

type ctxKey struct{}

// WithResultOverride attaches a per-call canned result that the next
// Scan made on ctx-derived contexts will return. Used by handler tests
// to assert end-to-end mapping without mocking the provider directly.
func WithResultOverride(ctx context.Context, result aivision.ScanResult) context.Context {
	return context.WithValue(ctx, ctxKey{}, override{result: result})
}

// WithErrorOverride attaches a per-call canned error. Subsequent Scans
// on ctx-derived contexts return (nil, err) and skip both defaults.
func WithErrorOverride(ctx context.Context, err error) context.Context {
	return context.WithValue(ctx, ctxKey{}, override{err: err})
}

func overrideFromContext(ctx context.Context) (override, bool) {
	v, ok := ctx.Value(ctxKey{}).(override)
	return v, ok
}

// DefaultResult is the canned ScanResult returned when no override is
// present. It covers every field the FE renders so visual review has
// something to assert against in screenshot tests.
func DefaultResult() aivision.ScanResult {
	return aivision.ScanResult{
		Fields: map[string]aivision.FieldGuess{
			aivision.FieldNameName:                  {Value: "Sample Wireless Headphones", Confidence: 0.92},
			aivision.FieldNameShortName:             {Value: "WH-Sample", Confidence: 0.78},
			aivision.FieldNameType:                  {Value: "electronics", Confidence: 0.95},
			aivision.FieldNameOriginalPrice:         {Value: 199.99, Confidence: 0.80},
			aivision.FieldNameOriginalPriceCurrency: {Value: "USD", Confidence: 0.85},
			aivision.FieldNameSerialNumber:          {Value: "SN-MOCK-001", Confidence: 0.70},
			aivision.FieldNameURLs:                  {Value: []string{"https://example.com/wh-sample"}, Confidence: 0.55},
			// Fixed purchase date keeps the canned result deterministic
			// across runs — using time.Now() made every test/screenshot
			// snapshot drift day-to-day.
			aivision.FieldNamePurchaseDate:      {Value: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), Confidence: 0.50},
			aivision.FieldNameWarrantyExpiresAt: {Value: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), Confidence: 0.45},
			aivision.FieldNameComments:          {Value: "Black over-ear wireless headphones with active noise cancellation.", Confidence: 0.65},
			aivision.FieldNameTags:              {Value: []string{"audio", "headphones", "wireless"}, Confidence: 0.60},
		},
		Warnings: []aivision.Warning{
			{Code: "low_confidence", Field: aivision.FieldNamePurchaseDate, Detail: "purchase date inferred from packaging design only"},
		},
		UsedTokens: 0,
		LatencyMS:  1,
	}
}

// MultiItemResult is the canned result the mock returns when the uploaded
// filename looks like a receipt/invoice — a stand-in for a multi-line invoice
// (two purchased products), so the item-chooser flow is demonstrable with the deterministic provider.
// Per the real-provider contract, Fields mirrors the first (most prominent)
// item. Dates are fixed so screenshots/e2e stay deterministic.
func MultiItemResult() aivision.ScanResult {
	purchased := time.Date(2024, 3, 14, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	espresso := map[string]aivision.FieldGuess{
		aivision.FieldNameName:                  {Value: "Espresso Machine", Confidence: 0.93},
		aivision.FieldNameShortName:             {Value: "Espresso", Confidence: 0.80},
		aivision.FieldNameType:                  {Value: "white_goods", Confidence: 0.88},
		aivision.FieldNameOriginalPrice:         {Value: 449.00, Confidence: 0.90},
		aivision.FieldNameOriginalPriceCurrency: {Value: "EUR", Confidence: 0.90},
		aivision.FieldNamePurchaseDate:          {Value: purchased, Confidence: 0.85},
		aivision.FieldNameComments:              {Value: "Purchased from Sample Store s.r.o.", Confidence: 0.70},
		aivision.FieldNameTags:                  {Value: []string{"coffee", "kitchen", "appliance"}, Confidence: 0.60},
	}
	frother := map[string]aivision.FieldGuess{
		aivision.FieldNameName:                  {Value: "Milk Frother", Confidence: 0.86},
		aivision.FieldNameShortName:             {Value: "Frother", Confidence: 0.75},
		aivision.FieldNameType:                  {Value: "electronics", Confidence: 0.80},
		aivision.FieldNameOriginalPrice:         {Value: 39.90, Confidence: 0.88},
		aivision.FieldNameOriginalPriceCurrency: {Value: "EUR", Confidence: 0.90},
		aivision.FieldNamePurchaseDate:          {Value: purchased, Confidence: 0.85},
		aivision.FieldNameComments:              {Value: "Purchased from Sample Store s.r.o.", Confidence: 0.70},
		aivision.FieldNameTags:                  {Value: []string{"coffee", "kitchen"}, Confidence: 0.60},
	}
	return aivision.ScanResult{
		Fields: espresso,
		Items: []aivision.ScanItem{
			{Fields: espresso},
			{Fields: frother},
		},
		UsedTokens: 0,
		LatencyMS:  1,
	}
}
