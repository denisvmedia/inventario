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
		{name: "viewer", role: models.GroupRoleViewer, wantErr: false},
		{name: "user", role: models.GroupRoleUser, wantErr: false},
		{name: "admin", role: models.GroupRoleAdmin, wantErr: false},
		{name: "owner", role: models.GroupRoleOwner, wantErr: false},
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
				TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
				GroupID:             "group-1",
				MemberUserID:        "user-1",
				Role:                models.GroupRoleAdmin,
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
				TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
				MemberUserID:        "user-1",
				Role:                models.GroupRoleAdmin,
			},
			wantErr: true,
		},
		{
			name: "missing member_user_id",
			m: models.GroupMembership{
				TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
				GroupID:             "group-1",
				Role:                models.GroupRoleAdmin,
			},
			wantErr: true,
		},
		{
			name: "invalid role",
			m: models.GroupMembership{
				TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
				GroupID:             "group-1",
				MemberUserID:        "user-1",
				Role:                "superadmin",
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

	// IsAdmin now reports role >= admin; owner counts as admin too.
	cases := []struct {
		role    models.GroupRole
		isAdmin bool
		isOwner bool
	}{
		{models.GroupRoleViewer, false, false},
		{models.GroupRoleUser, false, false},
		{models.GroupRoleAdmin, true, false},
		{models.GroupRoleOwner, true, true},
	}
	for _, tc := range cases {
		c.Run(string(tc.role), func(c *qt.C) {
			m := models.GroupMembership{Role: tc.role}
			c.Assert(m.IsAdmin(), qt.Equals, tc.isAdmin)
			c.Assert(m.IsOwner(), qt.Equals, tc.isOwner)
		})
	}
}

func TestGroupRole_AtLeast(t *testing.T) {
	c := qt.New(t)

	// Ranks: viewer(0) < user(1) < admin(2) < owner(3).
	type tc struct {
		have, min models.GroupRole
		want      bool
	}
	cases := []tc{
		{models.GroupRoleViewer, models.GroupRoleViewer, true},
		{models.GroupRoleViewer, models.GroupRoleUser, false},
		{models.GroupRoleUser, models.GroupRoleViewer, true},
		{models.GroupRoleUser, models.GroupRoleAdmin, false},
		{models.GroupRoleAdmin, models.GroupRoleAdmin, true},
		{models.GroupRoleAdmin, models.GroupRoleOwner, false},
		{models.GroupRoleOwner, models.GroupRoleAdmin, true},
		{models.GroupRoleOwner, models.GroupRoleOwner, true},
		// Unknown roles never satisfy AtLeast — fail-closed.
		{models.GroupRole("bogus"), models.GroupRoleViewer, false},
		{models.GroupRoleOwner, models.GroupRole("bogus"), false},
	}
	for _, k := range cases {
		c.Run(string(k.have)+">="+string(k.min), func(c *qt.C) {
			c.Assert(k.have.AtLeast(k.min), qt.Equals, k.want)
		})
	}
}

func TestGroupMembership_Validate_ReturnsError(t *testing.T) {
	c := qt.New(t)

	gm := &models.GroupMembership{}
	err := gm.Validate()
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.Equals, models.ErrMustUseValidateWithContext)
}
