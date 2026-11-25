package commodityquery_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/services/commodityquery"
)

func TestDigest(t *testing.T) {
	c := qt.New(t)
	ab := commodityquery.Digest([]string{"a", "b"})
	c.Assert(ab, qt.HasLen, 64)
	c.Assert(commodityquery.Digest([]string{"b", "a"}), qt.Equals, ab, qt.Commentf("order does not matter"))
	c.Assert(commodityquery.Digest([]string{"a", "c"}), qt.Not(qt.Equals), ab, qt.Commentf("same count, other item"))
	c.Assert(commodityquery.Digest([]string{"a"}), qt.Not(qt.Equals), ab)
	c.Assert(commodityquery.Digest(nil), qt.Equals, commodityquery.Digest([]string{}))

	in := []string{"b", "a"}
	commodityquery.Digest(in)
	c.Assert(in, qt.DeepEquals, []string{"b", "a"}, qt.Commentf("the input is not reordered"))
}
