package shared

import "net/url"

// RedactDSN masks the password embedded in a database DSN so credentials never
// leak into terminal history, CI logs, or screen-shared sessions. The username,
// host, database, and query parameters are preserved to keep diagnostic banners
// useful for troubleshooting; only the password component of the userinfo is
// replaced.
//
// If the DSN parses as a URL with a password, that password is masked. If it
// parses but carries no password, it is returned unchanged. If it does not parse
// as a URL at all, a safe placeholder is returned rather than echoing the raw
// value — a malformed DSN may still embed credentials we cannot reliably locate.
func RedactDSN(dsn string) string {
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
