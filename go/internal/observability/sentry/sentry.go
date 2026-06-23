// Package sentry is Inventario's thin wrapper around the Sentry Go SDK for
// error tracking (#844). It is a leaf package: it imports only the Sentry SDK
// (github.com/getsentry/sentry-go and its net/http subpackage), cleanenv, and
// the standard library. Wiring code in the bootstrap and apiserver packages
// calls Init, Middleware and CaptureError; nothing else touches the SDK
// directly.
//
// Everything is inert until Init binds a client from a non-empty SENTRY_DSN, so
// a deployment that does not configure Sentry pays effectively nothing:
// Middleware becomes a pass-through and CaptureError returns after a single
// atomic load.
package sentry

import (
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	sentrygo "github.com/getsentry/sentry-go"
	"github.com/ilyakaznacheev/cleanenv"
)

// enabled reports whether Init bound a real Sentry client. It gates the
// per-request Middleware work and CaptureError so the disabled path is free.
var enabled atomic.Bool

// Config holds the Sentry settings (#844). UNLIKE every other runtime setting,
// these are read with BARE env names (SENTRY_DSN, SENTRY_ENVIRONMENT,
// SENTRY_TRACES_SAMPLE_RATE) — NOT the INVENTARIO_RUN_ prefix the `run` section
// applies (#1618) — because the issue and Sentry's own conventions document the
// bare names. LoadConfig reads them directly via cleanenv.ReadEnv (no prefix
// wrapper) to honour that.
type Config struct {
	DSN              string  `env:"SENTRY_DSN"`
	Environment      string  `env:"SENTRY_ENVIRONMENT"`
	TracesSampleRate float64 `env:"SENTRY_TRACES_SAMPLE_RATE" env-default:"0.2"`
}

// LoadConfig reads the Sentry settings from the bare SENTRY_* environment
// variables. A missing DSN is not an error (it just means Sentry stays
// disabled); a non-nil error indicates a malformed value, e.g. a non-numeric
// SENTRY_TRACES_SAMPLE_RATE.
func LoadConfig() (Config, error) {
	var c Config
	if err := cleanenv.ReadEnv(&c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Init initialises the global Sentry client when cfg.DSN is set and returns a
// flush function to call on shutdown (it drains buffered events within the
// timeout, returning false if it could not). When cfg.DSN is empty it logs
// once, leaves Sentry disabled, and returns a no-op flush so callers can defer
// it unconditionally. A non-nil error means the DSN was malformed; the SDK does
// not perform any network I/O here, so this only fails on operator misconfig.
//
// Init is intended to be called EXACTLY ONCE during bootstrap, before request
// serving begins: it binds a process-global client and flips a package-global
// flag, with no teardown other than the returned flush. A second call would
// silently rebind the client.
func Init(cfg Config) (flush func(timeout time.Duration) bool, err error) {
	noop := func(time.Duration) bool { return true }
	if cfg.DSN == "" {
		slog.Info("Sentry disabled (SENTRY_DSN not set)")
		return noop, nil
	}
	// Data-exfiltration controls. SendDefaultPII is left at its false default,
	// which makes the SDK scrub sensitive request headers (Authorization, Cookie,
	// api-key, …), drop the cookie value, and omit the client IP — so the JWT
	// bearer and the httpOnly refresh-token cookie are safe. BUT the SDK attaches
	// the request BODY and QUERY STRING UNCONDITIONALLY (not gated by
	// SendDefaultPII), and its sensitive-header list misses Inventario's
	// X-CSRF-Token spelling. scrubRequestData (wired as both BeforeSend and
	// BeforeSendTransaction) strips those so no request secret reaches Sentry.
	// Do NOT enable SendDefaultPII without revisiting scrubRequestData.
	if initErr := sentrygo.Init(sentrygo.ClientOptions{
		Dsn:                   cfg.DSN,
		Environment:           cfg.Environment,
		EnableTracing:         cfg.TracesSampleRate > 0,
		TracesSampleRate:      cfg.TracesSampleRate,
		BeforeSend:            scrubRequestData,
		BeforeSendTransaction: scrubRequestData,
	}); initErr != nil {
		return noop, initErr
	}
	enabled.Store(true)
	slog.Info("Sentry error tracking enabled",
		"environment", cfg.Environment,
		"traces_sample_rate", cfg.TracesSampleRate)
	return sentrygo.Flush, nil
}

// csrfHeaderName is Inventario's CSRF request header (apiserver/csrf_middleware.go).
// The SDK's built-in sensitive-header scrubber covers "csrf-token" and
// "x-csrftoken" but NOT this exact spelling, so scrubRequestData drops it
// explicitly. It is duplicated here as a literal to keep this a leaf package
// (it must not import apiserver).
const csrfHeaderName = "X-CSRF-Token"

// scrubRequestData is the BeforeSend / BeforeSendTransaction hook. The Sentry
// SDK gates request headers and cookies behind SendDefaultPII (false here, so
// the Authorization bearer and refresh cookie are stripped), but it attaches
// the request BODY (event.Request.Data, up to 10 KiB) and QUERY STRING
// UNCONDITIONALLY. Inventario auth bodies carry plaintext credentials (password,
// TOTP/MFA code, reset/new password) and several query strings carry secrets
// (email-verify ?token=, signed-URL ?sig=). This hook clears those, and also
// drops the X-CSRF-Token header the SDK's scrubber misses. It never drops the
// event itself — only the sensitive sub-fields.
func scrubRequestData(event *sentrygo.Event, _ *sentrygo.EventHint) *sentrygo.Event {
	if event == nil || event.Request == nil {
		return event
	}
	event.Request.Data = ""
	event.Request.QueryString = ""
	for k := range event.Request.Headers {
		if strings.EqualFold(k, csrfHeaderName) {
			delete(event.Request.Headers, k)
		}
	}
	return event
}
