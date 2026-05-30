//go:build legacy_xml_backup

// DEPRECATED — LEGACY XML BACKUP CODE.
// Compiled ONLY under the `legacy_xml_backup` build tag; NOT in the default build.
// Implements the obsolete XML backup format that #534 replaced with the signed
// JSON `.inb` archive. Retained solely to be extracted into a separate repo as an
// XML-streaming proof-of-concept. Do not extend; do not couple new code to it.

package export

import (
	"context"
	"log/slog"
)

// cleanupDeletedExports is a deprecated no-op retained only so the
// legacy_xml_backup-tagged worker test keeps compiling. Exports now use
// immediate hard delete with file entities, so there is nothing to clean up.
// It lives in this build-tagged file (rather than the untagged worker.go) so the
// default `.inb` build does not carry dead code.
func (w *ExportWorker) cleanupDeletedExports(_ context.Context) {
	slog.Info("Export cleanup called but is no longer needed - exports use immediate deletion")
}
