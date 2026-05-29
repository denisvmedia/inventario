package apiserver

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

// parseTagKind is unexported, so this test lives in the apiserver package
// itself. Pins the wire contract for the required ?kind= param: commodity /
// file are accepted; empty or anything else 422s (item-tags and file-tags
// are separate entities, so there is no "all").
func TestParseTagKind(t *testing.T) {
	t.Run("empty string rejected — kind is required", func(t *testing.T) {
		c := qt.New(t)
		got, ok := parseTagKind("")
		c.Assert(ok, qt.IsFalse)
		c.Assert(got, qt.Equals, models.TagKindAny)
	})

	t.Run("commodity parses", func(t *testing.T) {
		c := qt.New(t)
		got, ok := parseTagKind("commodity")
		c.Assert(ok, qt.IsTrue)
		c.Assert(got, qt.Equals, models.TagKindCommodity)
	})

	t.Run("file parses", func(t *testing.T) {
		c := qt.New(t)
		got, ok := parseTagKind("file")
		c.Assert(ok, qt.IsTrue)
		c.Assert(got, qt.Equals, models.TagKindFile)
	})

	t.Run("whitespace is trimmed", func(t *testing.T) {
		c := qt.New(t)
		got, ok := parseTagKind("  commodity  ")
		c.Assert(ok, qt.IsTrue)
		c.Assert(got, qt.Equals, models.TagKindCommodity)
	})

	t.Run("unknown value rejected", func(t *testing.T) {
		c := qt.New(t)
		got, ok := parseTagKind("bogus")
		c.Assert(ok, qt.IsFalse)
		c.Assert(got, qt.Equals, models.TagKindAny)
	})

	t.Run("commodities plural rejected — wire contract is singular", func(t *testing.T) {
		c := qt.New(t)
		_, ok := parseTagKind("commodities")
		c.Assert(ok, qt.IsFalse)
	})
}
