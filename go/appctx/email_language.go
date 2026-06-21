package appctx

import "context"

// emailLanguageKey carries the recipient's resolved UI language (en/cs/ru)
// from a send site — where the user + tenant context is live and the
// recipient's `appearance.language` setting can be read — down to
// AsyncEmailService, which stamps it onto the enqueued email job. The job
// persists the language across the async queue boundary so the worker
// renders the localized template + subject. Empty means "unset" → the
// renderer falls back to English. (#2090)
const emailLanguageKey contextKey = "emailLanguage"

// WithEmailLanguage returns a context carrying the recipient's resolved
// email language (a short code such as "en"/"cs"/"ru"). An empty lang is
// ignored so callers can pass a best-effort lookup result unconditionally.
func WithEmailLanguage(ctx context.Context, lang string) context.Context {
	if lang == "" {
		return ctx
	}
	return context.WithValue(ctx, emailLanguageKey, lang)
}

// EmailLanguageFromContext returns the recipient email language set by
// WithEmailLanguage, or "" when none is set.
func EmailLanguageFromContext(ctx context.Context) string {
	if lang, ok := ctx.Value(emailLanguageKey).(string); ok {
		return lang
	}
	return ""
}
