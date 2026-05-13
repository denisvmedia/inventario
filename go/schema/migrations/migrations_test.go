package migrations_test

import (
	"testing"
	"testing/fstest"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/schema/migrations"
)

func TestMaxVersion_TakesHighestUpFile(t *testing.T) {
	c := qt.New(t)

	fsys := fstest.MapFS{
		"1779000000_first.up.sql":    {Data: []byte("--")},
		"1779000000_first.down.sql":  {Data: []byte("--")},
		"1779500000_third.up.sql":    {Data: []byte("--")},
		"1779500000_third.down.sql":  {Data: []byte("--")},
		"1779200000_second.up.sql":   {Data: []byte("--")},
		"1779200000_second.down.sql": {Data: []byte("--")},
	}

	maxVer, err := migrations.MaxVersion(fsys)
	c.Assert(err, qt.IsNil)
	c.Assert(maxVer, qt.Equals, int64(1779500000))
}

func TestMaxVersion_EmptyFSReturnsZero(t *testing.T) {
	c := qt.New(t)

	maxVer, err := migrations.MaxVersion(fstest.MapFS{})
	c.Assert(err, qt.IsNil)
	c.Assert(maxVer, qt.Equals, int64(0))
}

func TestMaxVersion_IgnoresUnversionedAndDownFiles(t *testing.T) {
	c := qt.New(t)

	// Only the .up.sql files contribute. .down.sql carrying the same prefix
	// would double-count; junk filenames (no integer prefix) are skipped so
	// a stray README never crashes the comparison.
	fsys := fstest.MapFS{
		"README.md":                   {Data: []byte("hi")},
		"1779100000_real.up.sql":      {Data: []byte("--")},
		"1779100000_real.down.sql":    {Data: []byte("--")},
		"not_a_version_prefix.up.sql": {Data: []byte("--")},
	}

	maxVer, err := migrations.MaxVersion(fsys)
	c.Assert(err, qt.IsNil)
	c.Assert(maxVer, qt.Equals, int64(1779100000))
}

// TestMaxVersion_EmbeddedFS_NonZero protects against the embed directive in
// migrations.go silently breaking — e.g. a refactor that renames `_sqldata`
// without updating the `//go:embed` line. If the embedded FS becomes empty,
// VerifySchemaUpToDate would silently degrade to a no-op everywhere.
func TestMaxVersion_EmbeddedFS_NonZero(t *testing.T) {
	c := qt.New(t)

	fsys, err := migrations.EmbeddedMigrationsFS()
	c.Assert(err, qt.IsNil)

	maxVer, err := migrations.MaxVersion(fsys)
	c.Assert(err, qt.IsNil)
	c.Assert(maxVer > 0, qt.IsTrue, qt.Commentf("expected at least one embedded migration"))
}
