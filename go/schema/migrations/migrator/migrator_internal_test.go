package migrator

import (
	"net/url"
	"testing"

	qt "github.com/frankban/quicktest"
)

// TestNew_StripsPGXPoolParamsFromStoredDSN guards the integration between the
// migrator and dsnutil: a DSN carrying pgxpool-only params (as compose hands
// us in POSTGRES_TEST_DSN) must arrive sanitised at every sql.Open("postgres",
// …) call inside this package, otherwise lib/pq forwards them to the server
// and the migrator dies on `42704 unrecognized configuration parameter`. We
// reach into the unexported field on purpose so this test still catches a
// future refactor that "forgets" to strip.
func TestNew_StripsPGXPoolParamsFromStoredDSN(t *testing.T) {
	c := qt.New(t)

	dsn := "postgres://user:pass@localhost:5432/db?sslmode=disable&pool_max_conns=1&pool_min_conns=1"
	m := New(dsn, nil)

	parsed, err := url.Parse(m.dbURL)
	c.Assert(err, qt.IsNil)

	q := parsed.Query()
	c.Assert(q.Has("pool_max_conns"), qt.IsFalse, qt.Commentf("pool_max_conns must be stripped to keep lib/pq happy"))
	c.Assert(q.Has("pool_min_conns"), qt.IsFalse, qt.Commentf("pool_min_conns must be stripped to keep lib/pq happy"))
	c.Assert(q.Get("sslmode"), qt.Equals, "disable", qt.Commentf("non-pool params must be preserved"))
}
