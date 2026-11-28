package registry_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/registry"
)

// url.Parse quotes the whole DSN in its error, password included. Parse
// keeps the reason and leaves the DSN out.
func TestConfig_ParseErrorLeavesTheDSNOut(t *testing.T) {
	c := qt.New(t)

	_, err := registry.Config("postgres://app:s3cret%zz@db:5432/inventario").Parse()

	c.Assert(err, qt.ErrorMatches, `invalid URL escape "%zz"`)
}
