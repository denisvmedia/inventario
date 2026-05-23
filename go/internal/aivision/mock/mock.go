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

// Scan implements aivision.Provider. It returns the per-call override
// from ctx when present, otherwise the constructor's defaults.
func (p *Provider) Scan(ctx context.Context, _ aivision.ScanRequest) (*aivision.ScanResult, error) {
	if override, ok := overrideFromContext(ctx); ok {
		if override.err != nil {
			return nil, override.err
		}
		// Make a copy so callers can't mutate the override across
		// subsequent calls.
		result := override.result
		return &result, nil
	}
	if p.defaultErr != nil {
		return nil, p.defaultErr
	}
	result := p.defaultResult
	return &result, nil
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
			aivision.FieldNamePurchaseDate:          {Value: time.Now().UTC().Format("2006-01-02"), Confidence: 0.50},
			aivision.FieldNameComments:              {Value: "Black over-ear wireless headphones with active noise cancellation.", Confidence: 0.65},
		},
		Warnings: []aivision.Warning{
			{Code: "low_confidence", Field: aivision.FieldNamePurchaseDate, Detail: "purchase date inferred from packaging design only"},
		},
		UsedTokens: 0,
		LatencyMS:  1,
	}
}
