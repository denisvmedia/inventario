package create_test

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/denisvmedia/inventario/models"
)

// TestMain lowers the bcrypt cost factor used by models.User.SetPassword
// for this test binary so the users-create CLI fixtures don't pay the
// production bcrypt.DefaultCost (~80ms / hash without -race, ~800ms
// with). Production CLI callers keep DefaultCost.
func TestMain(m *testing.M) {
	models.SetBcryptCostForTesting(nil, bcrypt.MinCost)
	os.Exit(m.Run())
}
