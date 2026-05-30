package openai_test

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
	"github.com/denisvmedia/inventario/internal/aivision/openai"
)

type fakeRoundTripper struct {
	handler func(req *http.Request) (*http.Response, error)
	last    *http.Request
	body    []byte
}

func (f *fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	_ = req.Body.Close()
	f.body = body
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

func happyResponse() string {
	inner := map[string]any{
		"fields": map[string]any{
			"name":           map[string]any{"value": "Test Item", "confidence": 0.92},
			"original_price": map[string]any{"value": 42.5, "confidence": 0.7},
		},
	}
	innerJSON, _ := json.Marshal(inner)
	payload := map[string]any{
		"choices": []map[string]any{
			{"message": map[string]any{"content": string(innerJSON)}},
		},
		"usage": map[string]any{"total_tokens": 200},
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
}

func TestOpenAIProvider_HappyPath(t *testing.T) {
	c := qt.New(t)

	client, rt := newFakeClient(func(_ *http.Request) (*http.Response, error) {
		return mustResponse(200, happyResponse()), nil
	})

	provider, err := openai.New(openai.Config{APIKey: "sk-test", HTTPClient: client})
	c.Assert(err, qt.IsNil)

	result, err := provider.Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{ContentType: "image/jpeg", Data: []byte("data")}},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(result.Fields["name"].Value, qt.Equals, "Test Item")
	c.Assert(result.UsedTokens, qt.Equals, 200)

	// Authorization header carries Bearer prefix; cannot be empty.
	c.Assert(rt.last.Header.Get("Authorization"), qt.Matches, `^Bearer .+`)
}

func TestOpenAIProvider_AuthFailure(t *testing.T) {
	c := qt.New(t)

	client, _ := newFakeClient(func(_ *http.Request) (*http.Response, error) {
		return mustResponse(401, `{"error":"unauthorized"}`), nil
	})

	provider, _ := openai.New(openai.Config{APIKey: "sk-test", HTTPClient: client})
	_, err := provider.Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{ContentType: "image/jpeg", Data: []byte("x")}},
	})
	c.Assert(err, qt.ErrorIs, aivision.ErrProviderAuth)
}

func TestOpenAIProvider_Throttled(t *testing.T) {
	c := qt.New(t)

	client, _ := newFakeClient(func(_ *http.Request) (*http.Response, error) {
		return mustResponse(429, `{"error":"rate"}`), nil
	})

	provider, _ := openai.New(openai.Config{APIKey: "sk-test", HTTPClient: client})
	_, err := provider.Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{ContentType: "image/jpeg", Data: []byte("x")}},
	})
	c.Assert(err, qt.ErrorIs, aivision.ErrProviderUnavailable)
}

func TestOpenAIProvider_ServerError(t *testing.T) {
	c := qt.New(t)

	client, _ := newFakeClient(func(_ *http.Request) (*http.Response, error) {
		return mustResponse(500, `{"error":"boom"}`), nil
	})

	provider, _ := openai.New(openai.Config{APIKey: "sk-test", HTTPClient: client})
	_, err := provider.Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{ContentType: "image/jpeg", Data: []byte("x")}},
	})
	c.Assert(err, qt.ErrorIs, aivision.ErrProviderUnavailable)
}

func TestOpenAIProvider_Timeout(t *testing.T) {
	c := qt.New(t)

	client, _ := newFakeClient(func(_ *http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})

	provider, _ := openai.New(openai.Config{APIKey: "sk-test", HTTPClient: client})
	_, err := provider.Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{ContentType: "image/jpeg", Data: []byte("x")}},
	})
	c.Assert(err, qt.ErrorIs, aivision.ErrProviderTimeout)
}

func TestOpenAIProvider_BadResponseShape(t *testing.T) {
	c := qt.New(t)

	client, _ := newFakeClient(func(_ *http.Request) (*http.Response, error) {
		return mustResponse(200, `{"choices":[]}`), nil
	})

	provider, _ := openai.New(openai.Config{APIKey: "sk-test", HTTPClient: client})
	_, err := provider.Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{ContentType: "image/jpeg", Data: []byte("x")}},
	})
	c.Assert(err, qt.ErrorIs, aivision.ErrProviderBadResponse)
}

func TestOpenAIProvider_EmptyAPIKey(t *testing.T) {
	c := qt.New(t)

	_, err := openai.New(openai.Config{})
	c.Assert(err, qt.ErrorIs, aivision.ErrProviderAuth)
}

func TestOpenAIProvider_MarshalTestPayload(t *testing.T) {
	c := qt.New(t)

	provider, err := openai.New(openai.Config{APIKey: "sk-test"})
	c.Assert(err, qt.IsNil)

	body, err := provider.MarshalTestPayload(aivision.ScanRequest{
		Photos:                []aivision.PhotoInput{{ContentType: "image/png", Data: []byte("photo-bytes")}},
		PreferredCurrencyCode: "EUR",
	})
	c.Assert(err, qt.IsNil)

	c.Assert(string(body), qt.Contains, "json_schema")
	c.Assert(string(body), qt.Contains, "image_url")
}
