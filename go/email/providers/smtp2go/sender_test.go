package smtp2go

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/email/sender"
)

type smtp2goRecordedRequest struct {
	method string
	path   string
	apiKey string
	body   []byte
}

func TestNew_RequiresAPIKey(t *testing.T) {
	c := qt.New(t)

	_, err := New(Config{})
	c.Assert(err, qt.IsNotNil)
}

func TestNew_InvalidBaseURL(t *testing.T) {
	c := qt.New(t)

	_, err := New(Config{
		APIKey:  "key",
		BaseURL: "://bad",
	})
	c.Assert(err, qt.IsNotNil)
}

func TestSend_Success(t *testing.T) {
	c := qt.New(t)

	recCh := make(chan smtp2goRecordedRequest, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()

		recCh <- smtp2goRecordedRequest{
			method: r.Method,
			path:   r.URL.Path,
			apiKey: r.Header.Get("X-Smtp2go-Api-Key"),
			body:   body,
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"request_id":"r1","data":{"succeeded":1,"failed":0,"failures":[],"email_id":"e1"}}`))
	}))
	defer srv.Close()

	s, err := New(Config{
		APIKey:     "key-1",
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	})
	c.Assert(err, qt.IsNil)

	msg := sender.Message{
		To:      "user@example.com",
		From:    "noreply@example.com",
		ReplyTo: "support@example.com",
		Subject: "hello",
		HTML:    "<p>hello</p>",
		Text:    "hello",
	}
	err = s.Send(context.Background(), msg)
	c.Assert(err, qt.IsNil)

	rec := <-recCh
	c.Assert(rec.method, qt.Equals, http.MethodPost)
	c.Assert(rec.path, qt.Equals, "/v3/email/send")
	c.Assert(rec.apiKey, qt.Equals, "key-1")

	var payload map[string]any
	err = json.Unmarshal(rec.body, &payload)
	c.Assert(err, qt.IsNil)

	c.Assert(payload["sender"], qt.Equals, msg.From)
	c.Assert(payload["subject"], qt.Equals, msg.Subject)
	c.Assert(payload["text_body"], qt.Equals, msg.Text)
	c.Assert(payload["html_body"], qt.Equals, msg.HTML)
	c.Assert(payload["to"], qt.DeepEquals, []any{msg.To})
	c.Assert(payload["custom_headers"], qt.HasLen, 1)
}

func TestSend_ProviderFailure(t *testing.T) {
	c := qt.New(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"request_id":"r1","data":{"succeeded":0,"failed":1,"failures":["bad recipient"]}}`))
	}))
	defer srv.Close()

	s, err := New(Config{
		APIKey:     "key-1",
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	})
	c.Assert(err, qt.IsNil)

	err = s.Send(context.Background(), sender.Message{
		To:      "user@example.com",
		From:    "noreply@example.com",
		Subject: "hello",
		HTML:    "<p>hello</p>",
		Text:    "hello",
	})
	c.Assert(err, qt.IsNotNil)
}

func TestSend_HTTPError(t *testing.T) {
	c := qt.New(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"request_id":"r1","data":{"error_code":"E_X","error":"bad"}}`))
	}))
	defer srv.Close()

	s, err := New(Config{
		APIKey:     "key-1",
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	})
	c.Assert(err, qt.IsNil)

	err = s.Send(context.Background(), sender.Message{
		To:      "user@example.com",
		From:    "noreply@example.com",
		Subject: "hello",
		HTML:    "<p>hello</p>",
		Text:    "hello",
	})
	c.Assert(err, qt.IsNotNil)
}
