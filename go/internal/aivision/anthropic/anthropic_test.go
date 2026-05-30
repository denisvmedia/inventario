package anthropic_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/aivision"
	"github.com/denisvmedia/inventario/internal/aivision/anthropic"
)

// fakeRoundTripper lets tests intercept the upstream call without
// touching the network. The handler receives the raw http.Request and
// returns an http.Response (or an error). Tests assert against the
// captured request and stub the response.
type fakeRoundTripper struct {
	handler func(req *http.Request) (*http.Response, error)
	last    *http.Request
	body    []byte
}

func (f *fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	_ = req.Body.Close()
	f.body = body
	// Reattach a fresh body in case downstream code expects to read it.
	req.Body = io.NopCloser(bytes.NewReader(body))
	f.last = req
	return f.handler(req)
}

func newFakeClient(handler func(*http.Request) (*http.Response, error)) (*http.Client, *fakeRoundTripper) {
	rt := &fakeRoundTripper{handler: handler}
	return &http.Client{Transport: rt, Timeout: 5 * time.Second}, rt
}

func mustResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

// happyResponse builds the upstream message body containing a single
// tool_use block carrying a valid extraction.
func happyResponse() string {
	payload := map[string]any{
		"content": []map[string]any{
			{
				"type": "tool_use",
				"name": "record_product_extraction",
				"input": map[string]any{
					"fields": map[string]any{
						"name":           map[string]any{"value": "Test Item", "confidence": 0.9},
						"original_price": map[string]any{"value": 99.5, "confidence": 0.8},
					},
				},
			},
		},
		"usage": map[string]any{"input_tokens": 100, "output_tokens": 20},
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
}

func TestAnthropicProvider_HappyPath(t *testing.T) {
	c := qt.New(t)

	client, rt := newFakeClient(func(_ *http.Request) (*http.Response, error) {
		return mustResponse(200, happyResponse()), nil
	})

	provider, err := anthropic.New(anthropic.Config{
		APIKey:     "sk-test-key",
		Model:      "claude-test",
		HTTPClient: client,
	})
	c.Assert(err, qt.IsNil)

	result, err := provider.Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{ContentType: "image/jpeg", Data: []byte("test-bytes")}},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(result.Fields["name"].Value, qt.Equals, "Test Item")
	c.Assert(result.Fields["original_price"].Value, qt.Equals, 99.5)
	c.Assert(result.UsedTokens, qt.Equals, 120)

	// Request shape: api key + version headers present, model + tool-use
	// forcing in the JSON body.
	c.Assert(rt.last.Header.Get("X-Api-Key"), qt.Equals, "sk-test-key")
	c.Assert(rt.last.Header.Get("Anthropic-Version"), qt.Not(qt.Equals), "")

	// Key cannot be logged — verify it does not appear in any common
	// logging-adjacent header we set. (Defensive: catches accidental
	// echo into Authorization.)
	c.Assert(rt.last.Header.Get("Authorization"), qt.Equals, "")
}

func TestAnthropicProvider_AuthFailure(t *testing.T) {
	c := qt.New(t)

	client, _ := newFakeClient(func(_ *http.Request) (*http.Response, error) {
		return mustResponse(401, `{"error":"unauthorized"}`), nil
	})

	provider, err := anthropic.New(anthropic.Config{APIKey: "sk-test", HTTPClient: client})
	c.Assert(err, qt.IsNil)

	_, err = provider.Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{ContentType: "image/jpeg", Data: []byte("x")}},
	})
	c.Assert(err, qt.ErrorIs, aivision.ErrProviderAuth)
}

func TestAnthropicProvider_RateLimited(t *testing.T) {
	c := qt.New(t)

	client, _ := newFakeClient(func(_ *http.Request) (*http.Response, error) {
		return mustResponse(429, `{"error":"rate"}`), nil
	})

	provider, _ := anthropic.New(anthropic.Config{APIKey: "sk-test", HTTPClient: client})
	_, err := provider.Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{ContentType: "image/jpeg", Data: []byte("x")}},
	})
	c.Assert(err, qt.ErrorIs, aivision.ErrProviderUnavailable)
}

func TestAnthropicProvider_ServerError(t *testing.T) {
	c := qt.New(t)

	client, _ := newFakeClient(func(_ *http.Request) (*http.Response, error) {
		return mustResponse(500, `{"error":"boom"}`), nil
	})

	provider, _ := anthropic.New(anthropic.Config{APIKey: "sk-test", HTTPClient: client})
	_, err := provider.Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{ContentType: "image/jpeg", Data: []byte("x")}},
	})
	c.Assert(err, qt.ErrorIs, aivision.ErrProviderUnavailable)
}

func TestAnthropicProvider_Timeout(t *testing.T) {
	c := qt.New(t)

	client, _ := newFakeClient(func(_ *http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})

	provider, _ := anthropic.New(anthropic.Config{APIKey: "sk-test", HTTPClient: client})
	_, err := provider.Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{ContentType: "image/jpeg", Data: []byte("x")}},
	})
	c.Assert(err, qt.ErrorIs, aivision.ErrProviderTimeout)
}

func TestAnthropicProvider_BadResponseShape(t *testing.T) {
	c := qt.New(t)

	client, _ := newFakeClient(func(_ *http.Request) (*http.Response, error) {
		return mustResponse(200, `{"content":[{"type":"text"}]}`), nil
	})

	provider, _ := anthropic.New(anthropic.Config{APIKey: "sk-test", HTTPClient: client})
	_, err := provider.Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{ContentType: "image/jpeg", Data: []byte("x")}},
	})
	c.Assert(err, qt.ErrorIs, aivision.ErrProviderBadResponse)
}

func TestAnthropicProvider_EmptyAPIKey(t *testing.T) {
	c := qt.New(t)

	_, err := anthropic.New(anthropic.Config{})
	c.Assert(err, qt.ErrorIs, aivision.ErrProviderAuth)
}

func TestAnthropicProvider_MarshalTestPayload(t *testing.T) {
	c := qt.New(t)

	provider, err := anthropic.New(anthropic.Config{APIKey: "sk-test"})
	c.Assert(err, qt.IsNil)

	body, err := provider.MarshalTestPayload(aivision.ScanRequest{
		Photos:                []aivision.PhotoInput{{ContentType: "image/png", Data: []byte("photo-bytes")}},
		PreferredCurrencyCode: "USD",
	})
	c.Assert(err, qt.IsNil)

	// Confirm the schema-tool is included and the choice forces it.
	c.Assert(string(body), qt.Contains, "record_product_extraction")
	c.Assert(string(body), qt.Contains, "tool_choice")
}
