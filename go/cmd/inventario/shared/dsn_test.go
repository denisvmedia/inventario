package shared_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
)

func TestRedactDSN_MasksPassword(t *testing.T) {
	c := qt.New(t)

	got := shared.RedactDSN("postgres://user:s3cr3t@localhost:5432/inventario?sslmode=disable")

	c.Assert(got, qt.Equals, "postgres://user:xxxxxx@localhost:5432/inventario?sslmode=disable")
}

func TestRedactDSN_PreservesUsernameHostAndParams(t *testing.T) {
	c := qt.New(t)

	got := shared.RedactDSN("postgresql://admin:hunter2@db.internal:6543/app?pool_max_conns=10")

	c.Assert(got, qt.Contains, "admin")
	c.Assert(got, qt.Contains, "db.internal:6543")
	c.Assert(got, qt.Contains, "/app")
	c.Assert(got, qt.Contains, "pool_max_conns=10")
	c.Assert(got, qt.Not(qt.Contains), "hunter2")
}

func TestRedactDSN_NoPasswordReturnedUnchanged(t *testing.T) {
	c := qt.New(t)

	dsn := "postgres://user@localhost/inventario"

	c.Assert(shared.RedactDSN(dsn), qt.Equals, dsn)
}

func TestRedactDSN_NoUserinfoReturnedUnchanged(t *testing.T) {
	c := qt.New(t)

	dsn := "memory://"

	c.Assert(shared.RedactDSN(dsn), qt.Equals, dsn)
}

func TestRedactDSN_UnparseableNeverEchoesRaw(t *testing.T) {
	c := qt.New(t)

	// A control character makes url.Parse fail; the raw value (which could embed
	// credentials) must not be echoed back.
	got := shared.RedactDSN("postgres://user:s3cr3t@localhost/db\x7f")

	c.Assert(got, qt.Equals, "<redacted>")
	c.Assert(got, qt.Not(qt.Contains), "s3cr3t")
}
