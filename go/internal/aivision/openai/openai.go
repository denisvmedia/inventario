// Package openai implements aivision.Provider against OpenAI's Chat
// Completions API (gpt-4o by default). Structured output is requested
// via response_format=json_schema, guaranteeing the response is valid
// JSON matching the shared schema.
//
// The HTTP client is injectable so tests substitute a fake
// http.RoundTripper. The package NEVER logs the API key or the raw
// Authorization header.
package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/internal/aivision"
)

// Name is the stable identifier this provider reports for the audit
// table and configuration discriminator.
const Name = "openai"

const (
	defaultBaseURL   = "https://api.openai.com"
	endpointMessages = "/v1/chat/completions"
	schemaName       = "product_extraction"
)

// DefaultModel is the model used when no override is supplied via Config.
// Operators may override via AI_VISION_OPENAI_MODEL.
const DefaultModel = "gpt-4o"

// DefaultMaxTokens is the output-budget cap used when Config.MaxTokens is
// unset. A multi-line invoice (one ~10-field product per line) easily blows
// past the old 1024 cap and the JSON is truncated; 4096 holds well over a
// dozen products. It is only a ceiling — the model stops once done.
const DefaultMaxTokens = 4096

// Config carries the runtime knobs. APIKey is required; Model defaults
// to DefaultModel; BaseURL defaults to the public API; HTTPClient
// defaults to a 30s-timeout client (separate from the per-call context
// deadline). MaxTokens caps the model's output budget.
type Config struct {
	APIKey     string
	Model      string
	BaseURL    string
	HTTPClient *http.Client
	MaxTokens  int
}

// Provider is the OpenAI implementation of aivision.Provider.
type Provider struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
	maxTokens  int
}

// New constructs a Provider from cfg. Returns an error when APIKey is
// empty so the registry constructor can downgrade to disabled.
func New(cfg Config) (*Provider, error) {
	if cfg.APIKey == "" {
		return nil, errxtrace.Classify(aivision.ErrProviderAuth)
	}
	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		// Must exceed the service-layer per-call deadline (AI_VISION_TIMEOUT,
		// default 60s) so that context deadline is the binding one rather than
		// this client cap.
		httpClient = &http.Client{Timeout: 90 * time.Second}
	}
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	return &Provider{
		apiKey:     cfg.APIKey,
		model:      model,
		baseURL:    baseURL,
		httpClient: httpClient,
		maxTokens:  maxTokens,
	}, nil
}

// Name implements aivision.Provider.
func (*Provider) Name() string { return Name }

// Model implements aivision.Provider. Returns the resolved model id
// (the constructor falls back to DefaultModel when Config.Model is
// empty) so the audit row records the actual upstream variant.
func (p *Provider) Model() string { return p.model }

// Scan implements aivision.Provider via Chat Completions with
// json_schema response_format.
func (p *Provider) Scan(ctx context.Context, req aivision.ScanRequest) (*aivision.ScanResult, error) {
	if len(req.Photos) == 0 {
		return nil, errxtrace.Classify(aivision.ErrProviderBadResponse)
	}

	payload := p.buildPayload(req)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, errxtrace.Wrap("marshal openai request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+endpointMessages, bytes.NewReader(body))
	if err != nil {
		return nil, errxtrace.Wrap("build openai request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	start := time.Now()
	resp, err := p.httpClient.Do(httpReq)
	latency := time.Since(start)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, errxtrace.Classify(aivision.ErrProviderTimeout)
		}
		return nil, errxtrace.Classify(aivision.ErrProviderUnavailable)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errxtrace.Classify(aivision.ErrProviderUnavailable)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, errxtrace.Classify(aivision.ErrProviderAuth)
	}
	if resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusGatewayTimeout {
		return nil, errxtrace.Classify(aivision.ErrProviderTimeout)
	}
	if resp.StatusCode >= 400 {
		return nil, errxtrace.Classify(aivision.ErrProviderUnavailable)
	}

	result, err := parseResponse(respBody)
	if err != nil {
		return nil, err
	}
	result.LatencyMS = latency.Milliseconds()
	return result, nil
}

// openaiRequest is the JSON shape POSTed to /v1/chat/completions with
// json_schema response_format.
type openaiRequest struct {
	Model          string           `json:"model"`
	MaxTokens      int              `json:"max_tokens"`
	Messages       []messagePayload `json:"messages"`
	ResponseFormat responseFormat   `json:"response_format"`
}

type messagePayload struct {
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	ImageURL *imageURLBlock `json:"image_url,omitempty"`
	File     *fileBlock     `json:"file,omitempty"`
}

type imageURLBlock struct {
	URL string `json:"url"`
}

// fileBlock is the "file" content part used to pass a PDF document inline
// (#1983 Part B). FileData is a base64 data: URL just like image_url's URL;
// Filename is required by the API for inline file_data, so the provider
// always sends a non-empty name (see pdfFilename).
type fileBlock struct {
	Filename string `json:"filename"`
	FileData string `json:"file_data"`
}

type responseFormat struct {
	Type       string               `json:"type"`
	JSONSchema responseFormatSchema `json:"json_schema"`
}

type responseFormatSchema struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

// openaiResponse is the slice of the upstream response shape we read.
type openaiResponse struct {
	Choices []openaiChoice `json:"choices"`
	Usage   *openaiUsage   `json:"usage,omitempty"`
}

type openaiChoice struct {
	Message openaiResponseMessage `json:"message"`
}

type openaiResponseMessage struct {
	Content string `json:"content"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (p *Provider) buildPayload(req aivision.ScanRequest) *openaiRequest {
	content := make([]contentBlock, 0, len(req.Photos)+2)
	content = append(content, contentBlock{
		Type: "text",
		Text: aivision.UserPromptHeader(req),
	})
	for _, photo := range req.Photos {
		// A PDF rides in a "file" content part (inline base64 data URL);
		// an image rides in an "image_url" part. Both encode the bytes as
		// the same data: URL — only the wrapping part differs.
		if photo.IsPDF() {
			content = append(content, contentBlock{
				Type: "file",
				File: &fileBlock{
					Filename: pdfFilename(photo.Filename),
					FileData: dataURL(photo.ContentType, photo.Data),
				},
			})
			continue
		}
		content = append(content, contentBlock{
			Type: "image_url",
			ImageURL: &imageURLBlock{
				URL: dataURL(photo.ContentType, photo.Data),
			},
		})
	}

	systemMessage := messagePayload{
		Role: "system",
		Content: []contentBlock{
			{Type: "text", Text: aivision.SystemPrompt},
		},
	}
	userMessage := messagePayload{Role: "user", Content: content}

	return &openaiRequest{
		Model:     p.model,
		MaxTokens: p.maxTokens,
		Messages:  []messagePayload{systemMessage, userMessage},
		ResponseFormat: responseFormat{
			Type: "json_schema",
			JSONSchema: responseFormatSchema{
				Name: schemaName,
				// Strict mode is intentionally OFF. OpenAI strict mode
				// requires every property to be in `required` (and
				// optional values to use `["string", "null"]`), which
				// forces the model to emit every field key explicitly —
				// the opposite of our wire contract where absent keys
				// mean "no signal" and the FE renders the input blank.
				// ToScanResult drops unknown keys and demotes
				// type-mismatched values to warnings, so non-strict
				// output is safe to consume.
				Strict: false,
				Schema: aivision.ResponseSchema(),
			},
		},
	}
}

// parseResponse extracts the single JSON message and converts it to the
// shared ScanResult shape.
func parseResponse(body []byte) (*aivision.ScanResult, error) {
	var resp openaiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, errxtrace.Classify(aivision.ErrProviderBadResponse)
	}
	if len(resp.Choices) == 0 {
		return nil, errxtrace.Classify(aivision.ErrProviderBadResponse)
	}
	content := resp.Choices[0].Message.Content
	if content == "" {
		return nil, errxtrace.Classify(aivision.ErrProviderBadResponse)
	}
	result, err := aivision.ToScanResult([]byte(content))
	if err != nil {
		return nil, errxtrace.Classify(aivision.ErrProviderBadResponse)
	}
	if resp.Usage != nil {
		result.UsedTokens = resp.Usage.TotalTokens
	}
	return result, nil
}

// dataURL encodes raw bytes as the data:URL form image_url / file_data
// expect.
func dataURL(contentType string, data []byte) string {
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(data))
}

// pdfFilename returns a non-empty filename for a PDF file part. The
// upstream API wants a name alongside inline file_data; the original
// upload may not carry one (a multipart part without a filename, or the
// anonymous path), so fall back to a stable default.
func pdfFilename(name string) string {
	if strings.TrimSpace(name) == "" {
		return "document.pdf"
	}
	return name
}

// Compile-time check that the constructor result satisfies the
// Provider interface.
var _ aivision.Provider = (*Provider)(nil)

// init wires the openai provider into the aivision registry so callers
// can select it by name via aivision.NewProvider. Standard registry-
// pattern init; side-effect imports in the bootstrap layer bring this
// runtime registration into the binary.
//
//nolint:gochecknoinits // standard registry-pattern provider registration
func init() {
	aivision.RegisterProvider(Name, func(cfg aivision.ProviderConfig) (aivision.Provider, error) {
		return New(Config{
			APIKey:     cfg.OpenAIAPIKey,
			Model:      cfg.OpenAIModel,
			BaseURL:    cfg.OpenAIBaseURL,
			HTTPClient: cfg.HTTPClient,
			MaxTokens:  cfg.MaxTokens,
		})
	})
}

// MarshalTestPayload returns the JSON body the provider would POST for
// the given request, without performing any network I/O.
func (p *Provider) MarshalTestPayload(req aivision.ScanRequest) ([]byte, error) {
	return json.Marshal(p.buildPayload(req))
}
