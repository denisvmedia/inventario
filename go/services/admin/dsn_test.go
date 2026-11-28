package admin_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/cmd/inventario/shared"
	"go.5x5.cz/inventario/services/admin"
)

// The CLI prints this error, so it must not carry the DSN and its password.
func TestNewService_UnsupportedDSNErrorLeavesThePasswordOut(t *testing.T) {
	c := qt.New(t)

	_, err := admin.NewService(&shared.DatabaseConfig{DBDSN: "postgres://app:s3cret%zz@db:5432/inventario"})

	c.Assert(err, qt.ErrorMatches, `unsupported database type in DSN \(expected postgres://\)`)
	c.Assert(err.Error(), qt.Not(qt.Contains), "s3cret")
}
