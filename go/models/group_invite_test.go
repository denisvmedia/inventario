package models_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestGroupInvite_ValidateWithContext(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	futureTime := time.Now().Add(24 * time.Hour)

	tests := []struct {
		name    string
		invite  models.GroupInvite
		wantErr bool
	}{
		{
			name: "valid invite",
			invite: models.GroupInvite{
				TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-1"},
				GroupID:            "group-1",
				Token:              "some-token-value",
				CreatedBy:          "user-1",
				ExpiresAt:          futureTime,
			},
			wantErr: false,
		},
		{
			name: "missing tenant_id",
			invite: models.GroupInvite{
				GroupID:   "group-1",
				Token:     "some-token-value",
				CreatedBy: "user-1",
				ExpiresAt: futureTime,
			},
			wantErr: true,
		},
		{
			name: "missing group_id",
			invite: models.GroupInvite{
				TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-1"},
				Token:              "some-token-value",
				CreatedBy:          "user-1",
				ExpiresAt:          futureTime,
			},
			wantErr: true,
		},
		{
			name: "missing token",
			invite: models.GroupInvite{
				TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-1"},
				GroupID:            "group-1",
				CreatedBy:          "user-1",
				ExpiresAt:          futureTime,
			},
			wantErr: true,
		},
		{
			name: "missing created_by",
			invite: models.GroupInvite{
				TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-1"},
				GroupID:            "group-1",
				Token:              "some-token-value",
				ExpiresAt:          futureTime,
			},
			wantErr: true,
		},
		{
			name: "zero expires_at",
			invite: models.GroupInvite{
				TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-1"},
				GroupID:            "group-1",
				Token:              "some-token-value",
				CreatedBy:          "user-1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			err := tt.invite.ValidateWithContext(ctx)
			if tt.wantErr {
				c.Assert(err, qt.IsNotNil)
			} else {
				c.Assert(err, qt.IsNil)
			}
		})
	}
}

func TestGroupInvite_IsExpired(t *testing.T) {
	c := qt.New(t)

	notExpired := models.GroupInvite{ExpiresAt: time.Now().Add(1 * time.Hour)}
	c.Assert(notExpired.IsExpired(), qt.IsFalse)

	expired := models.GroupInvite{ExpiresAt: time.Now().Add(-1 * time.Hour)}
	c.Assert(expired.IsExpired(), qt.IsTrue)
}

func TestGroupInvite_IsUsed(t *testing.T) {
	c := qt.New(t)

	unused := models.GroupInvite{}
	c.Assert(unused.IsUsed(), qt.IsFalse)

	userID := "user-1"
	used := models.GroupInvite{UsedBy: &userID}
	c.Assert(used.IsUsed(), qt.IsTrue)
}

func TestGroupInvite_IsValid(t *testing.T) {
	c := qt.New(t)

	// Valid: not expired, not used
	valid := models.GroupInvite{ExpiresAt: time.Now().Add(1 * time.Hour)}
	c.Assert(valid.IsValid(), qt.IsTrue)

	// Invalid: expired
	expired := models.GroupInvite{ExpiresAt: time.Now().Add(-1 * time.Hour)}
	c.Assert(expired.IsValid(), qt.IsFalse)

	// Invalid: used
	userID := "user-1"
	used := models.GroupInvite{
		ExpiresAt: time.Now().Add(1 * time.Hour),
		UsedBy:    &userID,
	}
	c.Assert(used.IsValid(), qt.IsFalse)

	// Invalid: both expired and used
	expiredAndUsed := models.GroupInvite{
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		UsedBy:    &userID,
	}
	c.Assert(expiredAndUsed.IsValid(), qt.IsFalse)
}

func TestGenerateInviteToken(t *testing.T) {
	c := qt.New(t)

	token1, err := models.GenerateInviteToken()
	c.Assert(err, qt.IsNil)
	c.Assert(len(token1) > 0, qt.IsTrue)

	// Verify uniqueness
	token2, err := models.GenerateInviteToken()
	c.Assert(err, qt.IsNil)
	c.Assert(token1, qt.Not(qt.Equals), token2)
}

func TestGroupInvite_Validate_ReturnsError(t *testing.T) {
	c := qt.New(t)

	gi := &models.GroupInvite{}
	err := gi.Validate()
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.Equals, models.ErrMustUseValidateWithContext)
}
