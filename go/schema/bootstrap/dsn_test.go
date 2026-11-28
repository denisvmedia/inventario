package bootstrap_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/schema/bootstrap"
)

// A "/" in the password ends the host part early, so the rest of the
// password is the database name, which pgx quotes when it cannot connect.
// Apply refuses such a DSN before it connects.
func TestMigrator_Apply_RefusesAnAtAfterTheHost(t *testing.T) {
	c := qt.New(t)

	err := bootstrap.New().Apply(context.Background(), bootstrap.ApplyArgs{
		DSN: "postgres://app:12/tail@host/x",
		Template: bootstrap.TemplateData{
			Username:              "testuser",
			UsernameForMigrations: "testmigrator",
		},
	})

	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Equals,
		"the database URL has an @ after the host; percent-encode @ in the password or the database name as %40")
}
