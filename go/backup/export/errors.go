package export

import (
	"sync/atomic"

	"github.com/go-extras/errx"
)

var (
	ErrUnsupportedExportType = errx.NewSentinel("unsupported export type")

	// ErrDraftsTooLarge is returned when the group's open drafts take more
	// room in the archive than a restore accepts for one JSON document. The
	// export fails before anything is written, so it never publishes a
	// backup that cannot be restored.
	ErrDraftsTooLarge = errx.NewSentinel("the group's open drafts are too large for a backup; discard the drafts you no longer need and export again")
	// ErrIntakeDocumentsTooLarge is returned when the record of the
	// documents other systems delivered takes more than one archive member
	// may hold.
	ErrIntakeDocumentsTooLarge = errx.NewSentinel("the record of delivered documents is too large for a backup")
)

// defaultMaxJSONDocBytes is the largest JSON document a restore accepts
// (maxJSONDocBytes in backup/restore/processor). The two must stay equal.
const defaultMaxJSONDocBytes = 32 << 20

// maxJSONDocBytes is the limit the export checks, as an override of
// defaultMaxJSONDocBytes; zero means no override. Tests set it through
// SetMaxJSONDocBytesForTesting.
var maxJSONDocBytes atomic.Int64

// jsonDocLimit is the largest JSON member the export writes.
func jsonDocLimit() int64 {
	if n := maxJSONDocBytes.Load(); n > 0 {
		return n
	}
	return defaultMaxJSONDocBytes
}

type testingCleanup interface {
	Cleanup(func())
}

// SetMaxJSONDocBytesForTesting lowers the JSON document limit the export
// checks, so a test can reach it without writing 32 MiB of drafts. The
// previous limit comes back when t's test ends.
func SetMaxJSONDocBytesForTesting(t testingCleanup, n int64) {
	orig := maxJSONDocBytes.Load()
	maxJSONDocBytes.Store(n)
	t.Cleanup(func() { maxJSONDocBytes.Store(orig) })
}
