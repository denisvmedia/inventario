package integration_test

import (
	"flag"
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"go.5x5.cz/inventario/internal/pgtest"
	"go.5x5.cz/inventario/models"
)

// TestMain does two things for the whole suite.
//
// It lowers the bcrypt cost factor used by models.User.SetPassword. The
// integration tests seed many users to exercise cross-tenant isolation; at
// production bcrypt.DefaultCost each fixture costs ~80ms (~800ms with -race).
// The semantics under test (RLS isolation, ownership boundaries) are
// independent of the cost factor, so MinCost is safe and cuts wall-clock by
// ~10x.
//
// It also provides the PostgreSQL these tests run against. Every one of them
// used to skip unless POSTGRES_TEST_DSN was set, which meant they ran in one
// CI lane and nowhere else — not in `go test ./...`, not on a developer's
// machine. pgtest.Start honours that variable when it is set and otherwise
// starts an embedded server, so the suite runs everywhere (#1953). Short mode
// still skips: see pgtest.SkipIfShort.
func TestMain(m *testing.M) {
	models.SetBcryptCostForTesting(nil, bcrypt.MinCost)

	// m.Run parses the flags itself, but testing.Short panics before that
	// happens, and the decision to start a server has to be made first.
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}

	dsn, stop := pgtest.Start()
	os.Setenv(pgtest.DSNEnv, dsn)
	code := m.Run()
	stop()
	os.Exit(code)
}
