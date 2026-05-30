package metrics_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/metrics"
)

func TestParseSQLVerb(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{"select", "SELECT * FROM commodities", "select"},
		{"insert", "INSERT INTO areas (id) VALUES ($1)", "insert"},
		{"update", "UPDATE files SET path = $1", "update"},
		{"delete", "DELETE FROM tenants WHERE id = $1", "delete"},
		{"begin", "BEGIN", "begin"},
		{"commit", "COMMIT", "commit"},
		{"rollback", "ROLLBACK", "rollback"},
		{"with cte", "WITH t AS (SELECT 1) SELECT * FROM t", "with"},
		{"leading whitespace", "   \n\t SELECT 1", "select"},
		{"leading line comment", "-- a comment\nSELECT 1", "select"},
		{"leading paren", "(SELECT 1)", "select"},
		{"lowercase normalization", "sElEcT 1", "select"},
		{"set local role is other", "SET LOCAL ROLE app_user", "other"},
		{"empty is other", "", "other"},
		{"whitespace only is other", "   \n  ", "other"},
		{"comment only is other", "-- nothing here", "other"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(metrics.ParseSQLVerb(tc.sql), qt.Equals, tc.want)
		})
	}
}
