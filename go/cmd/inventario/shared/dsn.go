package shared

import "github.com/denisvmedia/inventario/schema/dsnutil"

// RedactDSN masks the password embedded in a database DSN so credentials never
// leak into terminal history, CI logs, or screen-shared sessions.
//
// It is a thin alias for [dsnutil.Redact]: the schema layer needs the same
// masking for its own status banner and cannot import a cmd package, so the
// implementation lives next to the other DSN helpers.
func RedactDSN(dsn string) string {
	return dsnutil.Redact(dsn)
}
