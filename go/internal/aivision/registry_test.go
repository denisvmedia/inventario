package aivision_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/aivision"
	_ "github.com/denisvmedia/inventario/internal/aivision/anthropic" // register provider
	_ "github.com/denisvmedia/inventario/internal/aivision/mock"      // register provider
	_ "github.com/denisvmedia/inventario/internal/aivision/openai"    // register provider
)

func TestNewProvider_DisabledByEmptyName(t *testing.T) {
	c := qt.New(t)

	_, err := aivision.NewProvider(aivision.ProviderConfig{Name: ""})
	c.Assert(err, qt.ErrorIs, aivision.ErrProviderDisabled)
}

func TestNewProvider_DisabledByNoneName(t *testing.T) {
	c := qt.New(t)

	_, err := aivision.NewProvider(aivision.ProviderConfig{Name: "none"})
	c.Assert(err, qt.ErrorIs, aivision.ErrProviderDisabled)
}

func TestNewProvider_UnknownName(t *testing.T) {
	c := qt.New(t)

	_, err := aivision.NewProvider(aivision.ProviderConfig{Name: "definitely-not-a-provider"})
	c.Assert(err, qt.ErrorIs, aivision.ErrProviderUnknown)
}

func TestNewProvider_Mock(t *testing.T) {
	c := qt.New(t)

	provider, err := aivision.NewProvider(aivision.ProviderConfig{Name: "mock"})
	c.Assert(err, qt.IsNil)
	c.Assert(provider, qt.IsNotNil)
	c.Assert(provider.Name(), qt.Equals, "mock")
}

func TestNewProvider_AnthropicMissingKey(t *testing.T) {
	c := qt.New(t)

	_, err := aivision.NewProvider(aivision.ProviderConfig{Name: "anthropic"})
	c.Assert(err, qt.ErrorIs, aivision.ErrProviderAuth)
}

func TestNewProvider_OpenAIMissingKey(t *testing.T) {
	c := qt.New(t)

	_, err := aivision.NewProvider(aivision.ProviderConfig{Name: "openai"})
	c.Assert(err, qt.ErrorIs, aivision.ErrProviderAuth)
}

func TestNewProvider_AnthropicWithKey(t *testing.T) {
	c := qt.New(t)

	provider, err := aivision.NewProvider(aivision.ProviderConfig{
		Name:            "anthropic",
		AnthropicAPIKey: "sk-test-key",
	})
	c.Assert(err, qt.IsNil)
	c.Assert(provider.Name(), qt.Equals, "anthropic")
}

func TestNewProvider_OpenAIWithKey(t *testing.T) {
	c := qt.New(t)

	provider, err := aivision.NewProvider(aivision.ProviderConfig{
		Name:         "openai",
		OpenAIAPIKey: "sk-test-key",
	})
	c.Assert(err, qt.IsNil)
	c.Assert(provider.Name(), qt.Equals, "openai")
}

func TestAllFieldNames_Closed(t *testing.T) {
	c := qt.New(t)

	expected := []string{
		"name", "short_name", "type", "original_price", "original_price_currency",
		"serial_number", "urls", "purchase_date", "warranty_expires_at", "comments",
	}
	c.Assert(aivision.AllFieldNames, qt.DeepEquals, expected)
}
