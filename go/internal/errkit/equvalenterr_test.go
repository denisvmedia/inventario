package errkit_test

import (
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/errkit"
)

func TestWithEquivalents(t *testing.T) {
	c := qt.New(t)

	e := errors.New("test error")
	errEquiv1 := errors.New("test equiv error 1")
	errEquiv2 := &testError{}
	errNotEquiv := errors.New("test not equiv error")
	err := errkit.WithEquivalents(e, errEquiv1, errEquiv2)
	c.Assert(err, qt.ErrorIs, e)
	c.Assert(err, qt.ErrorIs, errEquiv1)
	c.Assert(err, qt.ErrorIs, errEquiv2)
	var equivErr *testError
	c.Assert(err, qt.ErrorAs, &equivErr)
	c.Assert(err, qt.Not(qt.ErrorIs), errNotEquiv)
}
