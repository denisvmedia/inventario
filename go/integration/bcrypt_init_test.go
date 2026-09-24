package integration_test

import (
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
// It also stops whatever PostgreSQL the suite started. Every test here used
// to skip unless POSTGRES_TEST_DSN was set, which meant they ran in one CI
// lane and nowhere else — not in `go test ./...`, not on a developer's
// machine. testDSN asks pgtest, which honours that variable when it is set
// and otherwise starts an embedded server on first use (#1953).
func TestMain(m *testing.M) {
	models.SetBcryptCostForTesting(nil, bcrypt.MinCost)

	code := m.Run()
	pgtest.Stop()
	os.Exit(code)
}
