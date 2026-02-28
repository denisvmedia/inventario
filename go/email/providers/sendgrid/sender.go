package sendgrid

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

// Config defines SendGrid API transport settings.
//
// New uses this config to build an HTTP client-backed transport targeting the
// Mail Send endpoint.
type Config struct {
	// APIKey is the bearer token used for SendGrid API authentication.
	APIKey string
	// BaseURL overrides the API host (primarily for tests/proxies).
	BaseURL string
	// HTTPClient optionally overrides the default timeout-configured client.
	HTTPClient *http.Client
}

// Sender delivers email through the SendGrid Mail Send API.
//
// It translates sender.Message into the provider JSON schema and treats
// non-2xx responses as hard send failures.
type Sender struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// New creates a SendGrid sender from config and validates required fields.
func New(cfg Config) (*Sender, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("sendgrid provider requires SENDGRID_API_KEY")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	baseURL, err := normalizeBaseURL(cfg.BaseURL, "https://api.sendgrid.com")
	if err != nil {
		return nil, err
	}
	return &Sender{
		apiKey:     strings.TrimSpace(cfg.APIKey),
		baseURL:    baseURL,
		httpClient: client,
	}, nil
}

// Send executes one HTTP POST to /v3/mail/send.
//
// Request construction is deterministic and stateless; retry policy remains the
// responsibility of the caller.
func (s *Sender) Send(ctx context.Context, message sender.Message) error {
	payload := map[string]any{
		"personalizations": []any{
			map[string]any{
				"to": []any{
					map[string]any{"email": message.To},
				},
			},
		},
		"from":    map[string]any{"email": message.From},
		"subject": message.Subject,
		"content": []any{
			map[string]any{"type": "text/plain", "value": message.Text},
			map[string]any{"type": "text/html", "value": message.HTML},
		},
	}
	if strings.TrimSpace(message.ReplyTo) != "" {
		payload["reply_to"] = map[string]any{"email": message.ReplyTo}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sendgrid payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/v3/mail/send", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create sendgrid request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req) // #nosec G704 -- request URL is built from validated provider base URL configured by trusted runtime settings.
	if err != nil {
		return fmt.Errorf("sendgrid request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	rawBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
	return fmt.Errorf("sendgrid request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(rawBody)))
}

func normalizeBaseURL(rawBaseURL, defaultBaseURL string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(rawBaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid sendgrid base URL %q: %w", baseURL, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid sendgrid base URL %q: scheme and host are required", baseURL)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("invalid sendgrid base URL %q: unsupported scheme %q", baseURL, parsed.Scheme)
	}

	return parsed.String(), nil
}
