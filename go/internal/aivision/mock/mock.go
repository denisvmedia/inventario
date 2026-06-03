// Package mock provides a deterministic, network-free Provider used by
// unit tests and e2e runs. The default canned ScanResult exercises every
// field the FE renders so screenshot/visual review always has something
// to display; callers that need a specific shape attach an override via
// WithResultOverride(ctx, result).
package mock

import (
	"context"
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
func (p *Provider) Scan(ctx context.Context, _ aivision.ScanRequest) (*aivision.ScanResult, error) {
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
	result := cloneScanResult(p.defaultResult)
	return &result, nil
}

// cloneScanResult returns a deep copy of src. Fields (a map) and
// Warnings (a slice) are reallocated; FieldGuess.Value is left as-is
// because the concrete types we return are all immutable (string,
// float64, []string with its own allocated backing array — see the
// per-field clone for the slice case).
func cloneScanResult(src aivision.ScanResult) aivision.ScanResult {
	dst := aivision.ScanResult{
		UsedTokens: src.UsedTokens,
		LatencyMS:  src.LatencyMS,
	}
	if src.Fields != nil {
		dst.Fields = make(map[string]aivision.FieldGuess, len(src.Fields))
		for k, v := range src.Fields {
			dst.Fields[k] = aivision.FieldGuess{
				Value:      cloneFieldValue(v.Value),
				Confidence: v.Confidence,
			}
		}
	}
	if src.Warnings != nil {
		dst.Warnings = make([]aivision.Warning, len(src.Warnings))
		copy(dst.Warnings, src.Warnings)
	}
	return dst
}

// cloneFieldValue defensively copies the few slice-typed values we know
// about (currently only []string for the urls field). Scalar types pass
// through unchanged.
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
		},
		Warnings: []aivision.Warning{
			{Code: "low_confidence", Field: aivision.FieldNamePurchaseDate, Detail: "purchase date inferred from packaging design only"},
		},
		UsedTokens: 0,
		LatencyMS:  1,
	}
}
