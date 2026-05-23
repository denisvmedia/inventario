package admin_test

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/denisvmedia/inventario/models"
)

// TestMain lowers the bcrypt cost factor used by models.User.SetPassword
// for this test binary so the admin CLI fixtures that seed users via
// SetPassword don't pay the production bcrypt.DefaultCost (~80ms / hash
// without -race, ~800ms with) on every fixture. Production CLI callers
// keep DefaultCost.
func TestMain(m *testing.M) {
	models.SetBcryptCostForTesting(nil, bcrypt.MinCost)
	os.Exit(m.Run())
}
