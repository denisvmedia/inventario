package memory_test

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"go.5x5.cz/inventario/models"
)

// TestMain lowers the bcrypt cost factor used by models.User.SetPassword
// for this test binary. Memory registry tests seed users to exercise
// CRUD / recursive-delete paths; at production bcrypt.DefaultCost each
// fixture costs ~80ms (~800ms with -race).
func TestMain(m *testing.M) {
	models.SetBcryptCostForTesting(nil, bcrypt.MinCost)
	os.Exit(m.Run())
}
