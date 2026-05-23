// Package anthropic implements aivision.Provider against Anthropic's
// Messages API (claude-sonnet-4-6 by default). Structured output is
// requested via tool-use forcing: a single tool whose input_schema is
// the shared response schema is presented, and the model is forced to
// invoke it, guaranteeing the response is valid JSON matching the
// schema without prose wrapping.
//
// The HTTP client is injectable so tests substitute a fake
// http.RoundTripper. The package NEVER logs the API key or the raw
// Authorization header.
package anthropic

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/internal/aivision"
)

// Name is the stable identifier this provider reports for the audit
// table and configuration discriminator.
const Name = "anthropic"

const (
	defaultBaseURL    = "https://api.anthropic.com"
	defaultAPIVersion = "2023-06-01"
	endpointMessages  = "/v1/messages"
	toolName          = "record_product_extraction"
)

// DefaultModel is the model used when no override is supplied via Config.
// Operators may override via AI_VISION_ANTHROPIC_MODEL.
const DefaultModel = "claude-sonnet-4-6"

// Config carries the runtime knobs. APIKey is required; Model defaults
// to DefaultModel; BaseURL defaults to the public API; HTTPClient
// defaults to a 30s-timeout client (separate from the per-call context
// deadline so a stuck TLS handshake still trips). MaxTokens caps the
// model's output budget.
type Config struct {
	APIKey     string
	Model      string
	BaseURL    string
	HTTPClient *http.Client
	MaxTokens  int
}

// Provider is the Anthropic implementation of aivision.Provider.
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
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
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

// Scan implements aivision.Provider by issuing a single Messages request
// with tool-use forcing.
func (p *Provider) Scan(ctx context.Context, req aivision.ScanRequest) (*aivision.ScanResult, error) {
	if len(req.Photos) == 0 {
		return nil, errxtrace.Classify(aivision.ErrProviderBadResponse)
	}

	payload := p.buildPayload(req)

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, errxtrace.Wrap("marshal anthropic request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+endpointMessages, bytes.NewReader(body))
	if err != nil {
		return nil, errxtrace.Wrap("build anthropic request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("X-Api-Key", p.apiKey)
	httpReq.Header.Set("Anthropic-Version", defaultAPIVersion)

	start := time.Now()
	resp, err := p.httpClient.Do(httpReq)
	latency := time.Since(start)
	if err != nil {
		// Context-cancellation / deadline gets its own sentinel so the
		// service maps to a 504 instead of a generic 502.
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

// anthropicRequest is the JSON shape POSTed to /v1/messages with
// tool-use forcing.
type anthropicRequest struct {
	Model      string            `json:"model"`
	MaxTokens  int               `json:"max_tokens"`
	System     string            `json:"system,omitempty"`
	Messages   []messagePayload  `json:"messages"`
	Tools      []toolDeclaration `json:"tools"`
	ToolChoice toolChoicePayload `json:"tool_choice"`
}

type messagePayload struct {
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type   string            `json:"type"`
	Text   string            `json:"text,omitempty"`
	Source *imageSourceBlock `json:"source,omitempty"`
}

type imageSourceBlock struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type toolDeclaration struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type toolChoicePayload struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// anthropicResponse is the slice of the upstream response shape we read.
// We only need usage + content[].input on the tool_use block.
type anthropicResponse struct {
	Content []anthropicResponseBlock `json:"content"`
	Usage   *anthropicUsage          `json:"usage,omitempty"`
}

type anthropicResponseBlock struct {
	Type  string          `json:"type"`
	Input json.RawMessage `json:"input,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func (p *Provider) buildPayload(req aivision.ScanRequest) *anthropicRequest {
	content := make([]contentBlock, 0, len(req.Photos)+1)
	for _, photo := range req.Photos {
		content = append(content, contentBlock{
			Type: "image",
			Source: &imageSourceBlock{
				Type:      "base64",
				MediaType: photo.ContentType,
				Data:      base64.StdEncoding.EncodeToString(photo.Data),
			},
		})
	}
	content = append(content, contentBlock{
		Type: "text",
		Text: aivision.UserPromptHeader(req),
	})

	return &anthropicRequest{
		Model:     p.model,
		MaxTokens: p.maxTokens,
		System:    aivision.SystemPrompt,
		Messages: []messagePayload{
			{Role: "user", Content: content},
		},
		Tools: []toolDeclaration{
			{
				Name:        toolName,
				Description: "Record the structured product extraction the assistant inferred from the photos.",
				InputSchema: aivision.ResponseSchema(),
			},
		},
		ToolChoice: toolChoicePayload{Type: "tool", Name: toolName},
	}
}

// parseResponse walks the response content blocks for the tool_use call
// and converts the tool input JSON into the shared ScanResult shape.
func parseResponse(body []byte) (*aivision.ScanResult, error) {
	var resp anthropicResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, errxtrace.Classify(aivision.ErrProviderBadResponse)
	}
	for _, block := range resp.Content {
		if block.Type != "tool_use" || len(block.Input) == 0 {
			continue
		}
		result, err := aivision.ToScanResult(block.Input)
		if err != nil {
			return nil, errxtrace.Classify(aivision.ErrProviderBadResponse)
		}
		if resp.Usage != nil {
			result.UsedTokens = resp.Usage.InputTokens + resp.Usage.OutputTokens
		}
		return result, nil
	}
	return nil, errxtrace.Classify(aivision.ErrProviderBadResponse)
}

// Compile-time check that the constructor result satisfies the
// Provider interface.
var _ aivision.Provider = (*Provider)(nil)

// init wires the anthropic provider into the aivision registry so
// callers can select it by name via aivision.NewProvider. Standard
// registry-pattern init; side-effect imports in the bootstrap layer
// bring this runtime registration into the binary.
//
//nolint:gochecknoinits // standard registry-pattern provider registration
func init() {
	aivision.RegisterProvider(Name, func(cfg aivision.ProviderConfig) (aivision.Provider, error) {
		return New(Config{
			APIKey:     cfg.AnthropicAPIKey,
			Model:      cfg.AnthropicModel,
			BaseURL:    cfg.AnthropicBaseURL,
			HTTPClient: cfg.HTTPClient,
			MaxTokens:  cfg.MaxTokens,
		})
	})
}

// MarshalTestPayload returns the JSON body the provider would POST for
// the given request, without performing any network I/O. Used by the
// unit test to assert the wire shape.
func (p *Provider) MarshalTestPayload(req aivision.ScanRequest) ([]byte, error) {
	payload := p.buildPayload(req)
	return json.Marshal(payload)
}
