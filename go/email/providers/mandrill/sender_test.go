package mandrill

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/email/sender"
)

type mandrillRecordedRequest struct {
	method      string
	path        string
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

	recCh := make(chan mandrillRecordedRequest, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		recCh <- mandrillRecordedRequest{
			method:      r.Method,
			path:        r.URL.Path,
			contentType: r.Header.Get("Content-Type"),
			body:        body,
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"status":"sent"}]`))
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
		ReplyTo: "support@example.com",
		Subject: "hello",
		HTML:    "<p>hello</p>",
		Text:    "hello",
	})
	c.Assert(err, qt.IsNil)

	rec := <-recCh
	c.Assert(rec.method, qt.Equals, http.MethodPost)
	c.Assert(rec.path, qt.Equals, "/api/1.0/messages/send.json")
	c.Assert(rec.contentType, qt.Equals, "application/json")
	c.Assert(string(rec.body), qt.Contains, "\"key\":\"key-1\"")
	c.Assert(string(rec.body), qt.Contains, "\"Reply-To\":\"support@example.com\"")
}

func TestSend_Non2xx(t *testing.T) {
	c := qt.New(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid"))
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
	c.Assert(err.Error(), qt.Contains, "status 400")
}

func TestSend_RejectedByProvider(t *testing.T) {
	c := qt.New(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"status":"rejected","reject_reason":"hard-bounce"}]`))
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
	c.Assert(err.Error(), qt.Contains, "mandrill rejected email")
}
