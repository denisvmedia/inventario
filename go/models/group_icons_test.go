package models_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestIsValidGroupIcon(t *testing.T) {
	c := qt.New(t)

	c.Assert(models.IsValidGroupIcon(""), qt.IsFalse)
	c.Assert(models.IsValidGroupIcon("xyz"), qt.IsFalse)
	c.Assert(models.IsValidGroupIcon("fa:box"), qt.IsFalse)

	c.Assert(models.IsValidGroupIcon("📦"), qt.IsTrue)
	c.Assert(models.IsValidGroupIcon("🏠"), qt.IsTrue)
	c.Assert(models.IsValidGroupIcon("🏢"), qt.IsTrue)
	c.Assert(models.IsValidGroupIcon("🏡"), qt.IsTrue)
}

func TestValidGroupIcons_CategoriesPopulated(t *testing.T) {
	c := qt.New(t)

	c.Assert(len(models.ValidGroupIcons) > 0, qt.IsTrue)

	seen := make(map[string]struct{})
	for _, ic := range models.ValidGroupIcons {
		c.Assert(ic.Emoji, qt.Not(qt.Equals), "")
		c.Assert(ic.Label, qt.Not(qt.Equals), "")
		c.Assert(ic.Category, qt.Not(qt.Equals), "")

		if _, dup := seen[ic.Emoji]; dup {
			t.Fatalf("duplicate emoji in ValidGroupIcons: %s", ic.Emoji)
		}
		seen[ic.Emoji] = struct{}{}
	}
}
