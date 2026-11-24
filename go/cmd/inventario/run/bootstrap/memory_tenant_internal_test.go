package bootstrap

// White-box: seedMemoryDBDefaultTenant is unexported, and the tenant it
// seeds is only reachable through the registry it writes to.

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/memory"
)

func TestSeedMemoryDBDefaultTenant_RegistrationMode(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  string
		want models.RegistrationMode
	}{
		{"unset is closed", "", models.RegistrationModeClosed},
		{"open", "open", models.RegistrationModeOpen},
		{"approval", "approval", models.RegistrationModeApproval},
		{"an invalid value falls back to closed", "wide-open", models.RegistrationModeClosed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			t.Setenv("INVENTARIO_RUN_MEMORY_TENANT_SLUG", "test-org")
			t.Setenv("INVENTARIO_RUN_MEMORY_TENANT_REGISTRATION_MODE", tc.env)
			fs := memory.NewFactorySet()

			seedMemoryDBDefaultTenant("memory://", fs)

			tenant, err := fs.TenantRegistry.GetBySlug(context.Background(), "test-org")
			c.Assert(err, qt.IsNil)
			c.Assert(tenant.RegistrationMode, qt.Equals, tc.want)
		})
	}
}
