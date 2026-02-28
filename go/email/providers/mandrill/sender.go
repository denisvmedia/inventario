package mandrill

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

// Config defines Mailchimp Transactional (Mandrill) API settings.
//
// New uses this to configure endpoint selection and HTTP transport behavior.
type Config struct {
	// APIKey authenticates calls to the Mandrill API.
	APIKey string
	// BaseURL overrides the API host (useful for tests/proxies).
	BaseURL string
	// HTTPClient optionally overrides the default timeout-configured client.
	HTTPClient *http.Client
}

// Sender delivers email through the Mandrill API.
//
// It maps sender.Message into the /messages/send payload and checks both HTTP
// status and provider-level per-recipient result status.
type Sender struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// New creates a Mandrill sender from config and validates required fields.
func New(cfg Config) (*Sender, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("mandrill/mailchimp provider requires MANDRILL_API_KEY")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	baseURL, err := normalizeBaseURL(cfg.BaseURL, "https://mandrillapp.com")
	if err != nil {
		return nil, err
	}
	return &Sender{
		apiKey:     strings.TrimSpace(cfg.APIKey),
		baseURL:    baseURL,
		httpClient: client,
	}, nil
}

// Send performs one HTTP POST to /api/1.0/messages/send.json.
//
// Any provider rejection/invalid status is surfaced as an error so retry policy
// remains centralized in upstream orchestration.
func (s *Sender) Send(ctx context.Context, message sender.Message) error {
	type recipient struct {
		Email string `json:"email"`
		Type  string `json:"type"`
	}
	type requestPayload struct {
		Key     string `json:"key"`
		Message struct {
			HTML      string            `json:"html"`
			Text      string            `json:"text"`
			Subject   string            `json:"subject"`
			FromEmail string            `json:"from_email"`
			To        []recipient       `json:"to"`
			Headers   map[string]string `json:"headers,omitempty"`
		} `json:"message"`
	}

	payload := requestPayload{
		Key: s.apiKey,
	}
	payload.Message.HTML = message.HTML
	payload.Message.Text = message.Text
	payload.Message.Subject = message.Subject
	payload.Message.FromEmail = message.From
	payload.Message.To = []recipient{{Email: message.To, Type: "to"}}
	if strings.TrimSpace(message.ReplyTo) != "" {
		payload.Message.Headers = map[string]string{"Reply-To": message.ReplyTo}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal mandrill payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/api/1.0/messages/send.json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create mandrill request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req) // #nosec G704 -- request URL is built from validated provider base URL configured by trusted runtime settings.
	if err != nil {
		return fmt.Errorf("mandrill request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	rawBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mandrill request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(rawBody)))
	}

	var result []struct {
		Status       string `json:"status"`
		RejectReason string `json:"reject_reason"`
	}
	if err := json.Unmarshal(rawBody, &result); err == nil {
		for _, r := range result {
			if r.Status == "rejected" || r.Status == "invalid" {
				return fmt.Errorf("mandrill rejected email: %s", r.RejectReason)
			}
		}
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
		return "", fmt.Errorf("invalid mandrill base URL %q: %w", baseURL, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid mandrill base URL %q: scheme and host are required", baseURL)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("invalid mandrill base URL %q: unsupported scheme %q", baseURL, parsed.Scheme)
	}

	return parsed.String(), nil
}
