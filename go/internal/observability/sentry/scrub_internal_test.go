package sentry

import (
	"testing"

	qt "github.com/frankban/quicktest"
	sentrygo "github.com/getsentry/sentry-go"
)

// TestScrubRequestData_StripsBodyQueryAndCSRF pins the #844 PII fix: the SDK
// attaches the request body and query string to events unconditionally, so the
// BeforeSend/BeforeSendTransaction hook must clear them (plus the X-CSRF-Token
// header the SDK's own scrubber misses) while keeping the event and its
// non-sensitive headers.
func TestScrubRequestData_StripsBodyQueryAndCSRF(t *testing.T) {
	c := qt.New(t)

	event := &sentrygo.Event{
		Request: &sentrygo.Request{
			Data:        `{"email":"a@b.com","password":"hunter2"}`,
			QueryString: "token=secret-verify-token&sig=deadbeef",
			Headers: map[string]string{
				"X-Csrf-Token": "csrf-secret", // Go-canonicalised form of X-CSRF-Token
				"User-Agent":   "test-agent",
				"Content-Type": "application/json",
			},
		},
	}

	got := scrubRequestData(event, nil)

	c.Assert(got, qt.Equals, event) // event kept, not dropped
	c.Assert(got.Request.Data, qt.Equals, "")
	c.Assert(got.Request.QueryString, qt.Equals, "")
	_, hasCSRF := got.Request.Headers["X-Csrf-Token"]
	c.Assert(hasCSRF, qt.IsFalse)
	// Non-sensitive headers are retained for debugging value.
	c.Assert(got.Request.Headers["User-Agent"], qt.Equals, "test-agent")
	c.Assert(got.Request.Headers["Content-Type"], qt.Equals, "application/json")
}

func TestScrubRequestData_NilSafe(t *testing.T) {
	c := qt.New(t)

	c.Assert(scrubRequestData(nil, nil), qt.IsNil)

	ev := &sentrygo.Event{} // nil Request must not panic
	c.Assert(scrubRequestData(ev, nil), qt.Equals, ev)
}
