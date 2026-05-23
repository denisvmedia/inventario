package migrations_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/schema/migrations"
)

// TestEmbeddedMigrations_VersionNotInFuture is the CI guard that fails the
// build when any migration in `_sqldata/` carries a version prefix greater
// than the current Unix timestamp.
//
// Why this matters. ptah's migration version IDs are real UTC Unix
// timestamps; the migrator orders them ascending and uses the value as the
// authoritative "when did this land". A "fake-future" prefix (e.g. picking
// `1780000000` when wall-clock is `1779553000` to dodge a collision with a
// sibling in-flight migration) silently poisons every later migration:
// the next generator pass keeps stepping past wall-clock, the audit
// reasoning ("when was this row created?") drifts, and ordering against
// future real-time migrations becomes load-bearing on developers
// remembering the bump. The right fix is rebase + regenerate so the next
// real-time second is picked; never invent a future timestamp.
//
// This guard intentionally uses `time.Now().UTC()` rather than a
// build-time constant — a forward-pinned constant would slowly bit-rot
// the guard as wall-clock catches up. Whoever runs the test runs it
// against the *real* clock.
func TestEmbeddedMigrations_VersionNotInFuture(t *testing.T) {
	c := qt.New(t)

	fsys, err := migrations.EmbeddedMigrationsFS()
	c.Assert(err, qt.IsNil)

	names, err := migrations.ListMigrations(fsys)
	c.Assert(err, qt.IsNil)

	now := time.Now().UTC().Unix()
	for _, name := range names {
		// Each migration is a pair: `<version>_<name>.up.sql` and
		// `<version>_<name>.down.sql`. We inspect both because a
		// pair-mismatch (only one side carries a future prefix) would
		// be its own bug worth surfacing.
		if !strings.HasSuffix(name, ".up.sql") && !strings.HasSuffix(name, ".down.sql") {
			continue
		}
		prefix, _, ok := strings.Cut(name, "_")
		if !ok {
			c.Errorf("migration filename %q does not match <version>_<name>.{up,down}.sql", name)
			continue
		}
		v, parseErr := strconv.ParseInt(prefix, 10, 64)
		if parseErr != nil {
			c.Errorf("migration filename %q has non-numeric version prefix: %v", name, parseErr)
			continue
		}
		if v > now {
			c.Errorf(
				"migration %q version %d is in the future (now=%d). "+
					"Migration version IDs MUST be real UTC unix timestamps "+
					"≤ wall-clock now. Rebase + regenerate; never invent a "+
					"fake-future prefix. See AGENTS.md → Database Migrations.",
				name, v, now,
			)
		}
	}
}
