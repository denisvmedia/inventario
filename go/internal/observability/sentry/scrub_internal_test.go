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
			URL:         "https://host/api/v1/invites/secret-invite-token",
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
	c.Assert(got.Request.URL, qt.Equals, "") // invite token in the path is dropped
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

// TestClientOptions_WiresScrubAndPIIControls pins the #844 data-safety guarantee
// at the wiring level: that the scrubber is actually installed as BOTH
// BeforeSend and BeforeSendTransaction (a typo dropping one would leak the body
// on the transaction path with no other failing test) and that SendDefaultPII
// stays off. It also pins that tracing is enabled iff the sample rate is > 0.
func TestClientOptions_WiresScrubAndPIIControls(t *testing.T) {
	c := qt.New(t)

	opts := clientOptions(Config{DSN: "https://pub@example.com/1", Environment: "prod", TracesSampleRate: 0.2})

	c.Assert(opts.Dsn, qt.Equals, "https://pub@example.com/1")
	c.Assert(opts.Environment, qt.Equals, "prod")
	c.Assert(opts.TracesSampleRate, qt.Equals, 0.2)
	c.Assert(opts.EnableTracing, qt.IsTrue) // rate > 0
	c.Assert(opts.SendDefaultPII, qt.IsFalse)

	// Both hooks must be wired AND must scrub. Drive a secret-bearing event
	// through each and assert the sensitive fields are cleared — this fails if
	// either hook is nil or points at the wrong function.
	c.Assert(opts.BeforeSend, qt.IsNotNil)
	c.Assert(opts.BeforeSendTransaction, qt.IsNotNil)
	for _, hook := range []func(*sentrygo.Event, *sentrygo.EventHint) *sentrygo.Event{opts.BeforeSend, opts.BeforeSendTransaction} {
		ev := hook(&sentrygo.Event{Request: &sentrygo.Request{
			URL:         "https://host/api/v1/invites/secret",
			Data:        "password=hunter2",
			QueryString: "token=abc",
		}}, nil)
		c.Assert(ev.Request.Data, qt.Equals, "")
		c.Assert(ev.Request.QueryString, qt.Equals, "")
		c.Assert(ev.Request.URL, qt.Equals, "")
	}
}

func TestClientOptions_TracingDisabledWhenRateZero(t *testing.T) {
	c := qt.New(t)

	opts := clientOptions(Config{DSN: "https://pub@example.com/1", TracesSampleRate: 0})
	c.Assert(opts.EnableTracing, qt.IsFalse)
}
