package postgres_test

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"go.5x5.cz/inventario/internal/utcclock"
	"go.5x5.cz/inventario/models"
)

// TestMain lowers the bcrypt cost factor used by models.User.SetPassword
// for this test binary. Postgres registry tests seed users to exercise
// RLS / recursive-delete / group-purger paths; at production
// bcrypt.DefaultCost each fixture costs ~80ms (~800ms with -race).
func TestMain(m *testing.M) {
	// The binary under test pins UTC in main, so these tests have to as well
	// or they measure the developer's time zone rather than the code (#2594).
	utcclock.Pin()
	models.SetBcryptCostForTesting(nil, bcrypt.MinCost)
	os.Exit(m.Run())
}
