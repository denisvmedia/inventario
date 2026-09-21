package models_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
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

// An owning model reaches the embedded identity through
// `validation.Field(&m.TenantGroupAwareEntityID)`. The library
// dereferences that pointer before testing the value for
// ValidatableWithContext, so the methods have to be on the value type —
// with a pointer receiver the whole check is skipped and the model
// accepts a blank tenant, group or author.
func TestTenantGroupAwareEntityID_ValidatesThroughAnOwningModel(t *testing.T) {
	ctx := context.Background()

	complete := models.TenantGroupAwareEntityID{
		TenantID:        "tenant-1",
		GroupID:         "group-1",
		CreatedByUserID: "user-1",
	}

	tests := []struct {
		name    string
		mutate  func(*models.TenantGroupAwareEntityID)
		wantErr bool
	}{
		{name: "complete", mutate: func(*models.TenantGroupAwareEntityID) {}},
		{name: "blank tenant", mutate: func(i *models.TenantGroupAwareEntityID) { i.TenantID = "" }, wantErr: true},
		{name: "blank group", mutate: func(i *models.TenantGroupAwareEntityID) { i.GroupID = "" }, wantErr: true},
		{name: "blank author", mutate: func(i *models.TenantGroupAwareEntityID) { i.CreatedByUserID = "" }, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			ids := complete
			tt.mutate(&ids)

			// Every field this model requires beyond the identity is set,
			// so an error can only have come from the embedded check.
			owner := models.WarrantyReminder{
				TenantGroupAwareEntityID: ids,
				CommodityID:              "commodity-1",
				ThresholdDays:            int(models.WarrantyReminder30Days),
			}

			err := owner.ValidateWithContext(ctx)
			if tt.wantErr {
				c.Assert(err, qt.IsNotNil)
				return
			}
			c.Assert(err, qt.IsNil)
		})
	}
}

// The value type has to satisfy the interfaces, not only the pointer.
func TestTenantGroupAwareEntityID_ValueSatisfiesValidatable(t *testing.T) {
	c := qt.New(t)

	var entity models.TenantGroupAwareEntityID

	_, ok := any(entity).(interface{ Validate() error })
	c.Check(ok, qt.IsTrue, qt.Commentf("value type must implement Validate"))

	_, ok = any(entity).(interface {
		ValidateWithContext(context.Context) error
	})
	c.Check(ok, qt.IsTrue, qt.Commentf("value type must implement ValidateWithContext"))

	// The context-free form stays a refusal: the identity cannot be
	// checked without one.
	c.Check(entity.Validate(), qt.ErrorIs, models.ErrMustUseValidateWithContext)
}
