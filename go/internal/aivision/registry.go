package aivision

import (
	"net/http"

	errxtrace "github.com/go-extras/errx/stacktrace"
)

// ProviderConfig is the boot-time selector. It deliberately lives in
// the aivision package (rather than under cmd/.../bootstrap) so any
// future caller — CLI tool, worker, test harness — can wire a Provider
// from a typed config without duplicating the switch.
//
// Name selects which provider to instantiate. Recognised values:
//   - "" / "none": no provider; NewProvider returns nil + ErrProviderDisabled.
//   - "mock": the deterministic in-tree mock used by tests and e2e.
//   - "anthropic": Anthropic Claude vision.
//   - "openai": OpenAI GPT-4o vision.
//
// All other names yield ErrProviderUnknown so a typo in config fails
// loudly at boot rather than silently downgrading to disabled.
type ProviderConfig struct {
	Name string

	AnthropicAPIKey  string
	AnthropicModel   string
	AnthropicBaseURL string

	OpenAIAPIKey  string
	OpenAIModel   string
	OpenAIBaseURL string

	// MaxTokens caps the vendor's output budget; zero falls back to
	// each provider's default.
	MaxTokens int

	// HTTPClient is shared across vendor calls when non-nil. Useful in
	// tests to swap a RoundTripper without poking into individual
	// provider constructors.
	HTTPClient *http.Client
}

// providerFactory is the closed registration map of in-tree provider
// constructors. Adding a new provider means registering a constructor
// here AND threading new config fields onto ProviderConfig. Tests
// register a fake by name via RegisterTestProvider.
var providerFactory = map[string]func(ProviderConfig) (Provider, error){}

// RegisterProvider exposes the package-level provider map for in-tree
// constructors. Each concrete provider package (anthropic, openai,
// mock) calls this from its init() so the registry can resolve names
// without importing the providers directly (which would create a
// circular dependency).
func RegisterProvider(name string, ctor func(ProviderConfig) (Provider, error)) {
	if _, ok := providerFactory[name]; ok {
		panic("aivision: duplicate provider registration: " + name)
	}
	providerFactory[name] = ctor
}

// NewProvider returns a Provider for cfg or one of the sentinel errors
// (ErrProviderDisabled, ErrProviderUnknown). Callers should treat
// ErrProviderDisabled as "intentional 503 from the handler" and
// ErrProviderUnknown as "boot-time misconfiguration".
func NewProvider(cfg ProviderConfig) (Provider, error) {
	switch cfg.Name {
	case "", "none":
		return nil, errxtrace.Classify(ErrProviderDisabled)
	}
	ctor, ok := providerFactory[cfg.Name]
	if !ok {
		return nil, errxtrace.Classify(ErrProviderUnknown)
	}
	return ctor(cfg)
}
