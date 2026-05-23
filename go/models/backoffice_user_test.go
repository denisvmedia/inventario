package models_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

// validBackofficeUser builds a BackofficeUser that satisfies every
// rule in ValidateWithContext, so individual tests can mutate one
// field at a time to assert specific failure modes.
func validBackofficeUser() *models.BackofficeUser {
	return &models.BackofficeUser{
		Email: "ops@example.com",
		Name:  "Operator",
		Role:  models.BackofficeRolePlatformAdmin,
	}
}

func TestBackofficeUser_ValidateWithContext_HappyPath(t *testing.T) {
	c := qt.New(t)
	u := validBackofficeUser()
	c.Assert(u.ValidateWithContext(context.Background()), qt.IsNil)
}

// TestBackofficeUser_ValidateWithContext_RejectsUnknownRole guards the
// closed-set contract on BackofficeRole. The ozzo-validation library
// auto-invokes the field type's own Validate() (BackofficeRole.Validate
// in this case) when the field is referenced from validation.Field, so
// validation.Required + BackofficeRole.Validate together both enforce
// non-empty AND closed-set membership without an explicit validation.In
// rule. This test pins that behaviour so a future refactor of the role
// type or the validation library can't silently weaken it.
func TestBackofficeUser_ValidateWithContext_RejectsUnknownRole(t *testing.T) {
	c := qt.New(t)
	u := validBackofficeUser()
	u.Role = models.BackofficeRole("hacker")
	err := u.ValidateWithContext(context.Background())
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "support_agent")
	c.Assert(err.Error(), qt.Contains, "platform_admin")
}

func TestBackofficeUser_ValidateWithContext_RejectsMalformedEmail(t *testing.T) {
	c := qt.New(t)
	u := validBackofficeUser()
	u.Email = "not-an-email"
	err := u.ValidateWithContext(context.Background())
	c.Assert(err, qt.IsNotNil)
}
