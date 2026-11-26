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

	"go.5x5.cz/inventario/internal/aivision"
	"go.5x5.cz/inventario/internal/aivision/openai"
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

func TestOpenAIProvider_PDFFileBlock(t *testing.T) {
	c := qt.New(t)

	provider, err := openai.New(openai.Config{APIKey: "sk-test"})
	c.Assert(err, qt.IsNil)

	body, err := provider.MarshalTestPayload(aivision.ScanRequest{
		Photos: []aivision.PhotoInput{
			{Filename: "receipt.pdf", ContentType: "application/pdf", Data: []byte("%PDF-1.7")},
		},
	})
	c.Assert(err, qt.IsNil)

	// A PDF (#1983) rides in a "file" content part carrying a base64
	// data: URL plus the original filename — not an image_url part.
	c.Assert(string(body), qt.Contains, `"type":"file"`)
	c.Assert(string(body), qt.Contains, `"file_data":"data:application/pdf;base64,`)
	c.Assert(string(body), qt.Contains, `"filename":"receipt.pdf"`)
	c.Assert(string(body), qt.Not(qt.Contains), "image_url")
}

func TestOpenAIProvider_PDFFileBlock_DefaultFilename(t *testing.T) {
	c := qt.New(t)

	provider, err := openai.New(openai.Config{APIKey: "sk-test"})
	c.Assert(err, qt.IsNil)

	// A multipart part may arrive without a filename; the provider must
	// still send a non-empty name (the API requires one alongside inline
	// file_data).
	body, err := provider.MarshalTestPayload(aivision.ScanRequest{
		Photos: []aivision.PhotoInput{
			{ContentType: "application/pdf", Data: []byte("%PDF-1.7")},
		},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(string(body), qt.Contains, `"filename":"document.pdf"`)
}

// TestOpenAIProvider_LabelsEachFile: every file is preceded by its "File N"
// label, so the model can name the file a suggestion is about.
func TestOpenAIProvider_LabelsEachFile(t *testing.T) {
	c := qt.New(t)

	provider, err := openai.New(openai.Config{APIKey: "sk-test"})
	c.Assert(err, qt.IsNil)

	body, err := provider.MarshalTestPayload(aivision.ScanRequest{
		Photos: []aivision.PhotoInput{
			{Filename: "kettle.jpg", ContentType: "image/jpeg", Data: []byte("jpeg")},
			{Filename: "invoice.pdf", ContentType: "application/pdf", Data: []byte("%PDF-1.7")},
		},
	})
	c.Assert(err, qt.IsNil)

	var payload struct {
		Messages []struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
	}
	c.Assert(json.Unmarshal(body, &payload), qt.IsNil)
	c.Assert(payload.Messages, qt.HasLen, 2)
	parts := payload.Messages[1].Content
	c.Assert(parts, qt.HasLen, 5)
	c.Check(parts[0].Text, qt.Contains, "labeled File 0 to File 1")
	c.Check(parts[1].Text, qt.Equals, "File 0")
	c.Check(parts[2].Type, qt.Equals, "image_url")
	c.Check(parts[3].Text, qt.Equals, "File 1")
	c.Check(parts[4].Type, qt.Equals, "file")
}

// TestOpenAIProvider_ParsesFileSuggestions: the per-file suggestions of the
// message reach the result, checked against the files of the request.
func TestOpenAIProvider_ParsesFileSuggestions(t *testing.T) {
	c := qt.New(t)

	inner, err := json.Marshal(map[string]any{
		"items": []any{map[string]any{"name": map[string]any{"value": "Kettle", "confidence": 0.9}}},
		"files": []any{
			map[string]any{
				"index":         1,
				"document_type": map[string]any{"value": "photo", "confidence": 0.9},
				"title":         map[string]any{"value": "Kettle on the counter", "confidence": 0.7},
			},
			map[string]any{
				"index":         0,
				"document_type": map[string]any{"value": "receipt", "confidence": 0.8},
				"title":         map[string]any{"value": "Kaufland receipt, 2024", "confidence": 0.6},
			},
			map[string]any{"index": 1, "title": map[string]any{"value": "Twice", "confidence": 0.7}},
		},
	})
	c.Assert(err, qt.IsNil)
	raw, err := json.Marshal(map[string]any{"choices": []map[string]any{{"message": map[string]any{"content": string(inner)}}}})
	c.Assert(err, qt.IsNil)
	client, _ := newFakeClient(func(_ *http.Request) (*http.Response, error) {
		return mustResponse(200, string(raw)), nil
	})
	provider, err := openai.New(openai.Config{APIKey: "sk-test", HTTPClient: client})
	c.Assert(err, qt.IsNil)

	result, err := provider.Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{
			{Filename: "receipt.jpg", ContentType: "image/jpeg", Data: []byte("a")},
			{Filename: "kettle.jpg", ContentType: "image/jpeg", Data: []byte("b")},
		},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(result.Files, qt.DeepEquals, []aivision.FileGuess{
		{
			Index:        0,
			DocumentType: &aivision.FieldGuess{Value: "receipt", Confidence: 0.8},
			Title:        &aivision.FieldGuess{Value: "Kaufland receipt, 2024", Confidence: 0.6},
		},
		{
			Index:        1,
			DocumentType: &aivision.FieldGuess{Value: "photo", Confidence: 0.9},
			Title:        &aivision.FieldGuess{Value: "Kettle on the counter", Confidence: 0.7},
		},
	})
	c.Assert(result.Warnings, qt.HasLen, 1)
	c.Check(result.Warnings[0].Field, qt.Equals, aivision.FieldNameFiles)
}
