package smtp2go

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/denisvmedia/inventario/email/sender"
)

// Config defines SMTP2GO API transport settings.
//
// New uses this config to build an HTTP client-backed transport targeting the
// v3 /email/send endpoint.
type Config struct {
	// APIKey is sent in the X-Smtp2go-Api-Key header for authentication.
	APIKey string
	// BaseURL overrides the API host (primarily for tests/proxies).
	BaseURL string
	// HTTPClient optionally overrides the default timeout-configured client.
	HTTPClient *http.Client
}

// Sender delivers email through the SMTP2GO v3 send API.
//
// It translates sender.Message into the provider JSON schema and treats both
// non-2xx responses and provider-level failures (data.failed / data.error) as
// hard send failures so retry policy stays centralized upstream.
type Sender struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// New creates an SMTP2GO sender from config and validates required fields.
func New(cfg Config) (*Sender, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("smtp2go provider requires SMTP2GO_API_KEY")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	baseURL, err := normalizeBaseURL(cfg.BaseURL, "https://api.smtp2go.com")
	if err != nil {
		return nil, err
	}
	return &Sender{
		apiKey:     strings.TrimSpace(cfg.APIKey),
		baseURL:    baseURL,
		httpClient: client,
	}, nil
}

// Send performs one HTTP POST to /v3/email/send.
//
// SMTP2GO returns HTTP 200 with a per-request result object even for some
// rejections, so both the HTTP status and data.succeeded / data.failed are
// inspected before reporting success.
func (s *Sender) Send(ctx context.Context, message sender.Message) error {
	type customHeader struct {
		Header string `json:"header"`
		Value  string `json:"value"`
	}
	payload := map[string]any{
		"sender":    message.From,
		"to":        []string{message.To},
		"subject":   message.Subject,
		"text_body": message.Text,
		"html_body": message.HTML,
	}
	if strings.TrimSpace(message.ReplyTo) != "" {
		payload["custom_headers"] = []customHeader{{Header: "Reply-To", Value: message.ReplyTo}}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal smtp2go payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/v3/email/send", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create smtp2go request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Smtp2go-Api-Key", s.apiKey)

	resp, err := s.httpClient.Do(req) // #nosec G704 -- request URL is built from validated provider base URL configured by trusted runtime settings.
	if err != nil {
		return fmt.Errorf("smtp2go request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	rawBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("smtp2go request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(rawBody)))
	}

	var result struct {
		Data struct {
			Succeeded int    `json:"succeeded"`
			Failed    int    `json:"failed"`
			Error     string `json:"error"`
			ErrorCode string `json:"error_code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return fmt.Errorf("smtp2go response parse failed: %s", strings.TrimSpace(string(rawBody)))
	}
	if result.Data.Error != "" || result.Data.ErrorCode != "" {
		return fmt.Errorf("smtp2go rejected email: %s (%s)", result.Data.Error, result.Data.ErrorCode)
	}
	if result.Data.Failed > 0 || result.Data.Succeeded < 1 {
		return fmt.Errorf("smtp2go failed to send email (succeeded=%d failed=%d): %s", result.Data.Succeeded, result.Data.Failed, strings.TrimSpace(string(rawBody)))
	}
	return nil
}

func normalizeBaseURL(rawBaseURL, defaultBaseURL string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(rawBaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid smtp2go base URL %q: %w", baseURL, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid smtp2go base URL %q: scheme and host are required", baseURL)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("invalid smtp2go base URL %q: unsupported scheme %q", baseURL, parsed.Scheme)
	}

	return parsed.String(), nil
}
