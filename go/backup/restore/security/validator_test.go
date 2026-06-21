package security_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/backup/restore/security"
)

// newValidator builds a RestoreSecurityValidator around a JSON slog handler
// writing into the returned buffer so log assertions can inspect emitted
// records.
func newValidator() (*security.RestoreSecurityValidator, *bytes.Buffer) {
	logBuf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return security.NewRestoreSecurityValidator(logger), logBuf
}

// logRecords parses the captured JSON log buffer into a slice of decoded
// records so tests can assert on individual emitted attributes.
func logRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		must.Assert(json.Unmarshal([]byte(line), &rec))
		out = append(out, rec)
	}
	return out
}

func TestLogUnauthorizedAttempt_EmitsWarnRecord(t *testing.T) {
	c := qt.New(t)
	validator, logBuf := newValidator()

	validator.LogUnauthorizedAttempt(context.Background(), security.UnauthorizedAttempt{
		UserID:         "attacker-id",
		TargetEntityID: "target-123",
		EntityType:     "commodity",
		Operation:      "restore_link_files",
		AttemptType:    "cross_user_access",
	})

	recs := logRecords(t, logBuf)
	c.Assert(recs, qt.HasLen, 1)
	c.Assert(recs[0]["level"], qt.Equals, "WARN")
	c.Assert(recs[0]["msg"], qt.Equals, "Unauthorized entity access attempt")
	c.Assert(recs[0]["user_id"], qt.Equals, "attacker-id")
	c.Assert(recs[0]["target_entity_id"], qt.Equals, "target-123")
	c.Assert(recs[0]["entity_type"], qt.Equals, "commodity")
	c.Assert(recs[0]["attempt_type"], qt.Equals, "cross_user_access")
	c.Assert(recs[0]["operation"], qt.Equals, "restore_link_files")
}
