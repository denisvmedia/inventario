package postgres_test

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"go.5x5.cz/inventario/internal/pgtest"
	"go.5x5.cz/inventario/internal/utcclock"
	"go.5x5.cz/inventario/models"
)

// TestMain does three things for this suite.
//
// It lowers the bcrypt cost factor used by models.User.SetPassword. These
// tests seed users to exercise RLS / recursive-delete / group-purger paths;
// at production bcrypt.DefaultCost each fixture costs ~80ms (~800ms with
// -race).
//
// It pins UTC, because the binary under test pins it in main and otherwise
// these tests measure the developer's time zone rather than the code (#2594).
//
// It stops whatever PostgreSQL the suite started. This is the package that
// covers row-level security and the SQL the memory registry never runs, and
// all of it skipped unless POSTGRES_TEST_DSN was set — so it ran in one CI
// lane and nowhere else. skipIfNoPostgreSQL now asks pgtest, which honours
// that variable when it is set and otherwise starts an embedded server on
// first use (#1953).
func TestMain(m *testing.M) {
	utcclock.Pin()
	models.SetBcryptCostForTesting(nil, bcrypt.MinCost)

	code := m.Run()
	pgtest.Stop()
	os.Exit(code)
}
