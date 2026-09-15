// Package dsnutil contains small helpers for normalising PostgreSQL DSNs
// across the inventario schema layer (migrator, bootstrap).
package dsnutil

import (
	"net/url"
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

// Redact masks the password embedded in a database DSN so credentials never
// leak into terminal history, CI logs, or screen-shared sessions. The username,
// host, database, and query parameters are preserved to keep diagnostic banners
// useful for troubleshooting; only the password component of the userinfo is
// replaced.
//
// If the DSN parses as a URL with a password, that password is masked. If it
// parses but carries no password, it is returned unchanged. If it does not parse
// as a URL at all, a safe placeholder is returned rather than echoing the raw
// value — a malformed DSN may still embed credentials we cannot reliably locate.
func Redact(dsn string) string {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "<redacted>"
	}
	if parsed.User == nil {
		return dsn
	}
	if _, hasPassword := parsed.User.Password(); !hasPassword {
		return dsn
	}
	parsed.User = url.UserPassword(parsed.User.Username(), "xxxxxx")
	return parsed.String()
}
