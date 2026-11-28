// Package dsnutil contains small helpers for normalising PostgreSQL DSNs
// across the inventario schema layer (migrator, bootstrap).
package dsnutil

import (
	"net/url"
	"regexp"
	"strings"
)

// StripPGXPoolParams removes pgxpool-specific query parameters from a
// PostgreSQL DSN. pgx accepts pool_* keys (pool_max_conns, pool_min_conns,
// pool_max_conn_lifetime, …) only via pgxpool.ParseConfig, which discards
// them before opening the underlying connection. lib/pq and pgx.Connect /
// pgconn.ParseConfig do NOT strip them; they forward every unrecognised
// query param to the server as a startup_message parameter, and the server
// rejects pool_* with `42704 unrecognized configuration parameter`.
//
// Schema-layer code (migrator, bootstrap) opens connections via pq and
// pgx.Connect, so it must be handed a DSN free of pool_* keys. Stripping
// here lets callers pass the same DSN they use for pgxpool without each
// schema-layer entrypoint having to know which driver it ultimately uses.
//
// If the input is unparseable as a URL, it is returned unchanged.
func StripPGXPoolParams(dbURL string) string {
	parsed, err := url.Parse(dbURL)
	if err != nil {
		return dbURL
	}
	q := parsed.Query()
	stripped := false
	for k := range q {
		if strings.HasPrefix(k, "pool_") {
			q.Del(k)
			stripped = true
		}
	}
	if !stripped {
		return dbURL
	}
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

// Redact masks the password in a database DSN so credentials never leak into
// terminal history, CI logs, or screen-shared sessions. The username, host,
// database, and the other query parameters are preserved to keep diagnostic
// banners useful for troubleshooting.
//
// A URL DSN has the password of its userinfo and every query parameter whose
// name holds "password" (password, sslpassword) masked. A keyword DSN
// (`host=db password=s3cret`) has the value of every such keyword masked. A
// DSN that does not parse as a URL, or that has an "@" after the host, comes
// back as a placeholder: the userinfo ends with an "@", so an "@" in the
// path, the query or the fragment means a character the URL reserves cut the
// password short and the rest of it sits there.
func Redact(dsn string) string {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return redactedDSN
	}
	if parsed.Scheme == "" {
		return keywordPassword.ReplaceAllString(dsn, "${1}="+mask)
	}
	if parsed.Host != "" && strings.Contains(parsed.Path+parsed.RawQuery+parsed.Fragment, "@") {
		return redactedDSN
	}
	changed := false
	if parsed.User != nil {
		if _, hasPassword := parsed.User.Password(); hasPassword {
			parsed.User = url.UserPassword(parsed.User.Username(), mask)
			changed = true
		}
	}
	q := parsed.Query()
	for k := range q {
		if strings.Contains(strings.ToLower(k), "password") {
			q.Set(k, mask)
			parsed.RawQuery = q.Encode()
			changed = true
		}
	}
	if !changed {
		return dsn
	}
	return parsed.String()
}

const (
	mask        = "xxxxxx"
	redactedDSN = "<redacted>"
)

// keywordPassword matches a password keyword of a keyword DSN and its value:
// a single-quoted value with backslash escapes, or a run of characters up to
// the next unescaped space. An unterminated quote runs to the end.
var keywordPassword = regexp.MustCompile(`(?i)\b(\w*password)\s*=\s*(?:'(?:[^'\\]|\\.)*'?|(?:[^\s\\]|\\.)*)`)
