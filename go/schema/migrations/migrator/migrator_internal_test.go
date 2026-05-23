package migrator

import (
	"errors"
	"io"
	"log/slog"
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

// TestCompareSchemaVersion is the decision-arm test for the #1655 defense:
// after a successful MigrateUp the DB version should match the binary's
// embedded max version. Behind → fatal (the bug). Equal → pass. Ahead → warn.
func TestCompareSchemaVersion(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name      string
		dbVersion int64
		embedMax  int64
		wantErr   error
	}{
		{
			name:      "in sync — passes silently",
			dbVersion: 1779553010,
			embedMax:  1779553010,
		},
		{
			name:      "db behind binary — the #1655 footprint",
			dbVersion: 1779200000,
			embedMax:  1779553010,
			wantErr:   ErrSchemaLagsBinary,
		},
		{
			name:      "db ahead of binary — operator rolled the binary back, warn only",
			dbVersion: 1779553020,
			embedMax:  1779553010,
		},
		{
			name:      "both zero — fresh dev DB or empty fixture, no error",
			dbVersion: 0,
			embedMax:  0,
		},
		{
			name:      "fresh DB against real binary — db lags",
			dbVersion: 0,
			embedMax:  1779553010,
			wantErr:   ErrSchemaLagsBinary,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			err := compareSchemaVersion(logger, tt.dbVersion, tt.embedMax)
			if tt.wantErr == nil {
				c.Assert(err, qt.IsNil)
				return
			}
			c.Assert(err, qt.IsNotNil)
			c.Assert(errors.Is(err, tt.wantErr), qt.IsTrue, qt.Commentf("expected %v, got %v", tt.wantErr, err))
		})
	}
}
