package assert_test

import (
	"errors"
	"regexp"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/assert"
)

func TestNoError_WithNoError(t *testing.T) {
	c := qt.New(t)

	unexpectedError := (error)(nil)
	c.Assert(func() { assert.NoError(unexpectedError) }, qt.Not(qt.PanicMatches), ".*")
}

func TestNoError_WithError(t *testing.T) {
	c := qt.New(t)

	expectedError := errors.New("test error")

	c.Assert(func() { assert.NoError(expectedError) }, qt.PanicMatches, regexp.MustCompile(`.*test error.*`))
}
