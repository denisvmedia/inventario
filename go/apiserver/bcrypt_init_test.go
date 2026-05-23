package apiserver_test

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services"
)

// TestMain lowers the bcrypt cost used by models.User.SetPassword and
// services.MFAService.GenerateBackupCodes for the entire apiserver test
// binary. Without this, the package's combined wall-clock under
// `go test -race` exceeds the per-binary 10-minute panic timeout: the
// auth-MFA / back-office login / cross-plane impersonation suites
// collectively seed dozens of users at bcrypt.DefaultCost (~800ms each
// under -race) and mint backup codes 10-at-a-time at the same cost.
//
// Production callers are unaffected — the override is scoped to this
// test binary's process. We pass nil for *testing.T because TestMain
// wants the cost lowered for the lifetime of the binary, not bound to
// a single test's Cleanup.
func TestMain(m *testing.M) {
	models.SetBcryptCostForTesting(nil, bcrypt.MinCost)
	services.SetBackupCodeBcryptCostForTesting(nil, bcrypt.MinCost)
	os.Exit(m.Run())
}
