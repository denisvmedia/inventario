package memory

// White-box: memberDeparture lists a member's Add item sessions and then
// deletes them one by one. Another request can delete a session in between,
// but that window cannot be reached through the exported API: the departure
// is wired inside NewFactorySet and runs to completion in one call. The test
// swaps in an intake store whose delete finds the session already gone.

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// racingIntakeFactory hands out intake registries that lose every delete to
// a concurrent request.
type racingIntakeFactory struct {
	registry.IntakeRegistryFactory
}

func (f racingIntakeFactory) CreateServiceRegistry() registry.IntakeRegistry {
	return racingIntake{IntakeRegistry: f.IntakeRegistryFactory.CreateServiceRegistry()}
}

type racingIntake struct {
	registry.IntakeRegistry
}

// Delete deletes the session as the other request does, then deletes it
// again, which finds nothing.
func (r racingIntake) Delete(ctx context.Context, id string) error {
	if err := r.IntakeRegistry.Delete(ctx, id); err != nil {
		return err
	}
	return r.IntakeRegistry.Delete(ctx, id)
}

func TestMemberDeparture_SessionDeletedMeanwhileDoesNotStopTheCleanup(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := NewFactorySet()
	const tenantID, groupID, userID = "tenant-a", "group-a", "member-a"

	intake, ok := fs.IntakeRegistryFactory.(*IntakeRegistryFactory)
	c.Assert(ok, qt.IsTrue)
	session := &models.IntakeSession{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("session-a", tenantID, groupID, userID),
		ClientKey:                "scan-1",
		Origin:                   models.IntakeOriginUpload,
	}
	intake.base.items.Set(session.ID, session)

	invite, err := fs.GroupInviteRegistry.Create(ctx, models.GroupInvite{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		GroupID:             groupID,
		Token:               "unused-invite",
		CreatedBy:           userID,
		ExpiresAt:           time.Now().Add(time.Hour),
	})
	c.Assert(err, qt.IsNil)

	tokens, ok := fs.APITokenRegistry.(*APITokenRegistry)
	c.Assert(ok, qt.IsTrue)
	tokens.tokens["token-a"] = models.APIToken{
		TenantUserAwareEntityID: models.WithTenantUserAwareEntityID("token-a", tenantID, userID),
		Name:                    "group",
		TokenHash:               "hash-a",
		Scope:                   models.APITokenScopeReadOnly,
		GroupID:                 new(groupID),
	}

	departure := newMemberDeparture(fs, tokens)
	departure.intake = racingIntakeFactory{IntakeRegistryFactory: fs.IntakeRegistryFactory}
	c.Assert(departure.run(ctx, tenantID, groupID, userID), qt.IsNil)

	_, err = fs.GroupInviteRegistry.GetByToken(ctx, invite.Token)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(tokens.tokens["token-a"].RevokedAt, qt.IsNotNil)
}
