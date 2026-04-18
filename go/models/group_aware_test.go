package models_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestTenantGroupAwareEntityID_Interfaces(t *testing.T) {
	c := qt.New(t)

	var entity models.TenantGroupAwareEntityID

	// Verify interface compliance
	var _ models.IDable = &entity
	var _ models.TenantAware = &entity
	var _ models.GroupAware = &entity
	var _ models.CreatedByUserAware = &entity
	var _ models.TenantGroupAwareIDable = &entity

	c.Assert(&entity, qt.IsNotNil)
}

func TestTenantGroupAwareEntityID_Getters_Setters(t *testing.T) {
	c := qt.New(t)

	entity := models.TenantGroupAwareEntityID{}

	entity.SetID("id-1")
	c.Assert(entity.GetID(), qt.Equals, "id-1")

	entity.SetTenantID("tenant-1")
	c.Assert(entity.GetTenantID(), qt.Equals, "tenant-1")

	entity.SetGroupID("group-1")
	c.Assert(entity.GetGroupID(), qt.Equals, "group-1")

	entity.SetCreatedByUserID("user-1")
	c.Assert(entity.GetCreatedByUserID(), qt.Equals, "user-1")
}

func TestTenantGroupAwareEntityID_ValidateWithContext(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	tests := []struct {
		name    string
		entity  models.TenantGroupAwareEntityID
		wantErr bool
	}{
		{
			name: "valid entity",
			entity: models.TenantGroupAwareEntityID{
				TenantID:        "tenant-1",
				GroupID:         "group-1",
				CreatedByUserID: "user-1",
			},
			wantErr: false,
		},
		{
			name: "missing tenant_id",
			entity: models.TenantGroupAwareEntityID{
				GroupID:         "group-1",
				CreatedByUserID: "user-1",
			},
			wantErr: true,
		},
		{
			name: "missing group_id",
			entity: models.TenantGroupAwareEntityID{
				TenantID:        "tenant-1",
				CreatedByUserID: "user-1",
			},
			wantErr: true,
		},
		{
			name: "missing created_by_user_id",
			entity: models.TenantGroupAwareEntityID{
				TenantID: "tenant-1",
				GroupID:  "group-1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			err := tt.entity.ValidateWithContext(ctx)
			if tt.wantErr {
				c.Assert(err, qt.IsNotNil)
			} else {
				c.Assert(err, qt.IsNil)
			}
		})
	}
}

func TestWithGroupID(t *testing.T) {
	c := qt.New(t)

	entity := &models.TenantGroupAwareEntityID{}
	result := models.WithGroupID("group-1", entity)
	c.Assert(result.GetGroupID(), qt.Equals, "group-1")
}

func TestWithCreatedByUserID(t *testing.T) {
	c := qt.New(t)

	entity := &models.TenantGroupAwareEntityID{}
	result := models.WithCreatedByUserID("user-1", entity)
	c.Assert(result.GetCreatedByUserID(), qt.Equals, "user-1")
}

func TestWithTenantGroupAwareEntityID(t *testing.T) {
	c := qt.New(t)

	entity := models.WithTenantGroupAwareEntityID("id-1", "tenant-1", "group-1", "user-1")
	c.Assert(entity.GetID(), qt.Equals, "id-1")
	c.Assert(entity.GetTenantID(), qt.Equals, "tenant-1")
	c.Assert(entity.GetGroupID(), qt.Equals, "group-1")
	c.Assert(entity.GetCreatedByUserID(), qt.Equals, "user-1")
}
