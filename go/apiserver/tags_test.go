package apiserver

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/registry"
)

// parseTagScope is unexported, so this test lives in the apiserver
// package itself. Pins the wire contract for ?scope= introduced in #1628:
// empty / commodity / file are accepted; anything else 422s.
func TestParseTagScope(t *testing.T) {
	t.Run("empty string returns TagScopeAny + ok", func(t *testing.T) {
		c := qt.New(t)
		got, ok := parseTagScope("")
		c.Assert(ok, qt.IsTrue)
		c.Assert(got, qt.Equals, registry.TagScopeAny)
	})

	t.Run("commodity parses", func(t *testing.T) {
		c := qt.New(t)
		got, ok := parseTagScope("commodity")
		c.Assert(ok, qt.IsTrue)
		c.Assert(got, qt.Equals, registry.TagScopeCommodity)
	})

	t.Run("file parses", func(t *testing.T) {
		c := qt.New(t)
		got, ok := parseTagScope("file")
		c.Assert(ok, qt.IsTrue)
		c.Assert(got, qt.Equals, registry.TagScopeFile)
	})

	t.Run("whitespace is trimmed", func(t *testing.T) {
		c := qt.New(t)
		got, ok := parseTagScope("  commodity  ")
		c.Assert(ok, qt.IsTrue)
		c.Assert(got, qt.Equals, registry.TagScopeCommodity)
	})

	t.Run("unknown value rejected", func(t *testing.T) {
		c := qt.New(t)
		got, ok := parseTagScope("bogus")
		c.Assert(ok, qt.IsFalse)
		c.Assert(got, qt.Equals, registry.TagScopeAny)
	})

	t.Run("commodities plural rejected — wire contract is singular", func(t *testing.T) {
		c := qt.New(t)
		_, ok := parseTagScope("commodities")
		c.Assert(ok, qt.IsFalse)
	})
}
