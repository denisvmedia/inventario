package postgres_test

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/denisvmedia/inventario/models"
)

// TestMain lowers the bcrypt cost factor used by models.User.SetPassword
// for this test binary. Postgres registry tests seed users to exercise
// RLS / recursive-delete / group-purger paths; at production
// bcrypt.DefaultCost each fixture costs ~80ms (~800ms with -race).
func TestMain(m *testing.M) {
	models.SetBcryptCostForTesting(nil, bcrypt.MinCost)
	os.Exit(m.Run())
}
