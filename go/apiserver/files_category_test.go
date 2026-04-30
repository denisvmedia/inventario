package apiserver

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

// parseFileCategoryParam is unexported, so this test lives in the apiserver
// package itself rather than apiserver_test. Keeps the helper internal while
// still pinning behaviour for the four legal values + the rejection paths.
func TestParseFileCategoryParam(t *testing.T) {
	t.Run("absent param returns nil", func(t *testing.T) {
		c := qt.New(t)
		got, err := parseFileCategoryParam(nil)
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.IsNil)
	})

	t.Run("empty string returns nil", func(t *testing.T) {
		c := qt.New(t)
		got, err := parseFileCategoryParam([]string{""})
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.IsNil)
	})

	t.Run("each valid value parses", func(t *testing.T) {
		c := qt.New(t)
		for _, valid := range models.ValidFileCategories {
			got, err := parseFileCategoryParam([]string{string(valid)})
			c.Assert(err, qt.IsNil)
			c.Assert(got, qt.IsNotNil)
			c.Assert(*got, qt.Equals, valid)
		}
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		c := qt.New(t)
		got, err := parseFileCategoryParam([]string{"warranty"})
		c.Assert(got, qt.IsNil)
		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Contains, "invalid category")
	})

	t.Run("multi-value rejected", func(t *testing.T) {
		c := qt.New(t)
		got, err := parseFileCategoryParam([]string{"photos", "invoices"})
		c.Assert(got, qt.IsNil)
		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Contains, "single value")
	})
}
