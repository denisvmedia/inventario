package seeddata_test

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/denisvmedia/inventario/models"
)

// TestMain lowers the bcrypt cost factor used by models.User.SetPassword
// for this test binary. The seed data tests hash many test-user
// passwords during fixture setup; at production bcrypt.DefaultCost each
// fixture costs ~80ms (~800ms with -race). Production seeding (the
// debug CLI) is unaffected — the override is process-scoped.
func TestMain(m *testing.M) {
	models.SetBcryptCostForTesting(nil, bcrypt.MinCost)
	os.Exit(m.Run())
}
