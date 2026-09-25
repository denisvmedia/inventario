package bootstrap_test

import (
	"os"
	"testing"

	"go.5x5.cz/inventario/internal/pgtest"
)

// TestMain stops whatever PostgreSQL this package's tests started. pgtest
// starts one on first use — POSTGRES_TEST_DSN when it is set, an embedded
// server otherwise (#1953).
func TestMain(m *testing.M) {
	code := m.Run()
	pgtest.Stop()
	os.Exit(code)
}
