package sendgrid

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

type sendgridRecordedRequest struct {
	method      string
	path        string
	auth        string
	contentType string
	body        []byte
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

	recCh := make(chan sendgridRecordedRequest, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()

		recCh <- sendgridRecordedRequest{
			method:      r.Method,
			path:        r.URL.Path,
			auth:        r.Header.Get("Authorization"),
			contentType: r.Header.Get("Content-Type"),
			body:        body,
		}
		w.WriteHeader(http.StatusAccepted)
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
	c.Assert(rec.path, qt.Equals, "/v3/mail/send")
	c.Assert(rec.auth, qt.Equals, "Bearer key-1")
	c.Assert(rec.contentType, qt.Equals, "application/json")

	var payload map[string]any
	err = json.Unmarshal(rec.body, &payload)
	c.Assert(err, qt.IsNil)

	c.Assert(payload["subject"], qt.Equals, "hello")
	c.Assert(payload["reply_to"], qt.IsNotNil)
}

func TestSend_Non2xx(t *testing.T) {
	c := qt.New(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream down"))
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
	c.Assert(err.Error(), qt.Contains, "status 502")
	c.Assert(err.Error(), qt.Contains, "upstream down")
}
