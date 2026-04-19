package models_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestGroupRole_Validate(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name    string
		role    models.GroupRole
		wantErr bool
	}{
		{name: "admin", role: models.GroupRoleAdmin, wantErr: false},
		{name: "user", role: models.GroupRoleUser, wantErr: false},
		{name: "invalid", role: "superadmin", wantErr: true},
		{name: "empty", role: "", wantErr: true},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			err := tt.role.Validate()
			c.Assert(err != nil, qt.Equals, tt.wantErr)
		})
	}
}

func TestGroupMembership_ValidateWithContext(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	tests := []struct {
		name    string
		m       models.GroupMembership
		wantErr bool
	}{
		{
			name: "valid membership",
			m: models.GroupMembership{
				TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-1"},
				GroupID:            "group-1",
				MemberUserID:       "user-1",
				Role:               models.GroupRoleAdmin,
			},
			wantErr: false,
		},
		{
			name: "missing tenant_id",
			m: models.GroupMembership{
				GroupID:      "group-1",
				MemberUserID: "user-1",
				Role:         models.GroupRoleAdmin,
			},
			wantErr: true,
		},
		{
			name: "missing group_id",
			m: models.GroupMembership{
				TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-1"},
				MemberUserID:       "user-1",
				Role:               models.GroupRoleAdmin,
			},
			wantErr: true,
		},
		{
			name: "missing member_user_id",
			m: models.GroupMembership{
				TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-1"},
				GroupID:            "group-1",
				Role:               models.GroupRoleAdmin,
			},
			wantErr: true,
		},
		{
			name: "invalid role",
			m: models.GroupMembership{
				TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-1"},
				GroupID:            "group-1",
				MemberUserID:       "user-1",
				Role:               "superadmin",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			err := tt.m.ValidateWithContext(ctx)
			c.Assert(err != nil, qt.Equals, tt.wantErr)
		})
	}
}

func TestGroupMembership_IsAdmin(t *testing.T) {
	c := qt.New(t)

	admin := models.GroupMembership{Role: models.GroupRoleAdmin}
	c.Assert(admin.IsAdmin(), qt.IsTrue)

	user := models.GroupMembership{Role: models.GroupRoleUser}
	c.Assert(user.IsAdmin(), qt.IsFalse)
}

func TestGroupMembership_Validate_ReturnsError(t *testing.T) {
	c := qt.New(t)

	gm := &models.GroupMembership{}
	err := gm.Validate()
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.Equals, models.ErrMustUseValidateWithContext)
}
