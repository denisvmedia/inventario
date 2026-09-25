// Package logguard holds a source-level guard against logging a database DSN
// with its password still in it.
//
// A Postgres DSN carries the password in its userinfo, so a log line that
// takes one verbatim writes the database credential to stdout — and in the
// one place it happened (`slog.Error("Unknown registry", "dsn", dsn)`) it was
// on an error path, which is exactly where logs get pasted into an issue.
// The repository already carries two maskers for this, `shared.RedactDSN` and
// the userinfo rewrite in `logStartupInfo`, so the mistake is a forgotten
// call rather than a missing tool.
//
// The guard is a test rather than a linter because it needs no new tooling to
// run in CI, matching internal/validationguard.
package logguard
