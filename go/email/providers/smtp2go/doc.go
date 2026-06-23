// Package smtp2go provides an email sender backed by the SMTP2GO v3 HTTP API
// (POST /v3/email/send). It authenticates with the X-Smtp2go-Api-Key header and
// surfaces both transport (non-2xx) and provider-level (data.failed /
// data.error) failures so the caller's retry/backoff policy stays centralized.
package smtp2go
