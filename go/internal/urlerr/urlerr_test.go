package urlerr_test

import (
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/internal/urlerr"
)

func TestParse_ErrorLeavesTheURLOut(t *testing.T) {
	c := qt.New(t)

	u, err := urlerr.Parse("postgres://app:s3cret%zz@db:5432/inventario")

	c.Assert(u, qt.IsNil)
	c.Assert(err, qt.ErrorMatches, `invalid URL escape "%zz"`)
}

func TestParse_ValidURL(t *testing.T) {
	c := qt.New(t)

	u, err := urlerr.Parse("postgres://app:s3cret@db:5432/inventario")

	c.Assert(err, qt.IsNil)
	c.Assert(u.Host, qt.Equals, "db:5432")
}

func TestWithoutURL_OtherErrorPassesThrough(t *testing.T) {
	c := qt.New(t)

	other := errors.New("connection refused")

	c.Assert(urlerr.WithoutURL(other), qt.ErrorIs, other)
	c.Assert(urlerr.WithoutURL(nil), qt.IsNil)
}
