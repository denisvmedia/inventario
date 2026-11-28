package registry_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/internal/urlerr"
	"go.5x5.cz/inventario/registry"
)

// url.Parse quotes the whole DSN in its error, and with a "#" in the password
// the reason quotes the password as the port. Parse names the kind of
// problem and quotes nothing from the DSN.
func TestConfig_ParseErrorLeavesTheDSNOut(t *testing.T) {
	c := qt.New(t)

	_, err := registry.Config("postgres://app:s3cret#tail@db:5432/inventario").Parse()

	var urlErr *urlerr.Error
	c.Assert(err, qt.ErrorAs, &urlErr)
	c.Assert(urlErr.Reason, qt.Equals, "invalid port after host")
	c.Assert(err.Error(), qt.Not(qt.Contains), "s3cret")
}
