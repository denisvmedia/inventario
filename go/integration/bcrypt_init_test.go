package integration_test

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/denisvmedia/inventario/models"
)

// TestMain lowers the bcrypt cost factor used by models.User.SetPassword
// for this test binary. The integration tests seed many users to
// exercise cross-tenant isolation; at production bcrypt.DefaultCost each
// fixture costs ~80ms (~800ms with -race). The semantics under test
// (RLS isolation, ownership boundaries) are independent of the bcrypt
// cost factor, so MinCost is safe and cuts wall-clock by ~10x.
func TestMain(m *testing.M) {
	models.SetBcryptCostForTesting(nil, bcrypt.MinCost)
	os.Exit(m.Run())
}
