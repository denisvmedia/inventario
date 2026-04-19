package models_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestLocationGroupStatus_Validate(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name    string
		status  models.LocationGroupStatus
		wantErr bool
	}{
		{name: "active", status: models.LocationGroupStatusActive, wantErr: false},
		{name: "pending_deletion", status: models.LocationGroupStatusPendingDeletion, wantErr: false},
		{name: "invalid", status: "invalid_status", wantErr: true},
		{name: "empty", status: "", wantErr: true},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			err := tt.status.Validate()
			c.Assert(err != nil, qt.Equals, tt.wantErr)
		})
	}
}

func TestLocationGroup_ValidateWithContext(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	validSlug := "abcdefghijklmnopqrstuv" // 22 chars

	tests := []struct {
		name    string
		group   models.LocationGroup
		wantErr bool
	}{
		{
			name: "valid group",
			group: models.LocationGroup{
				TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-1"},
				Slug:               validSlug,
				Name:               "My Group",
				Status:             models.LocationGroupStatusActive,
				CreatedBy:          "user-1",
				MainCurrency:       models.Currency("USD"),
			},
			wantErr: false,
		},
		{
			name: "missing name",
			group: models.LocationGroup{
				TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-1"},
				Slug:               validSlug,
				Status:             models.LocationGroupStatusActive,
				CreatedBy:          "user-1",
				MainCurrency:       models.Currency("USD"),
			},
			wantErr: true,
		},
		{
			name: "slug too short",
			group: models.LocationGroup{
				TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-1"},
				Slug:               "short",
				Name:               "My Group",
				Status:             models.LocationGroupStatusActive,
				CreatedBy:          "user-1",
				MainCurrency:       models.Currency("USD"),
			},
			wantErr: true,
		},
		{
			name: "missing tenant_id",
			group: models.LocationGroup{
				Slug:         validSlug,
				Name:         "My Group",
				Status:       models.LocationGroupStatusActive,
				MainCurrency: models.Currency("USD"),
				CreatedBy:    "user-1",
			},
			wantErr: true,
		},
		{
			name: "missing created_by",
			group: models.LocationGroup{
				TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-1"},
				Slug:               validSlug,
				Name:               "My Group",
				Status:             models.LocationGroupStatusActive,
				MainCurrency:       models.Currency("USD"),
			},
			wantErr: true,
		},
		{
			name: "missing main_currency",
			group: models.LocationGroup{
				TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-1"},
				Slug:               validSlug,
				Name:               "My Group",
				Status:             models.LocationGroupStatusActive,
				CreatedBy:          "user-1",
			},
			wantErr: true,
		},
		{
			name: "name too long",
			group: models.LocationGroup{
				TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-1"},
				Slug:               validSlug,
				Name:               string(make([]byte, 101)),
				Status:             models.LocationGroupStatusActive,
				CreatedBy:          "user-1",
				MainCurrency:       models.Currency("USD"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			err := tt.group.ValidateWithContext(ctx)
			c.Assert(err != nil, qt.Equals, tt.wantErr)
		})
	}
}

func TestLocationGroup_IsActive(t *testing.T) {
	c := qt.New(t)

	active := models.LocationGroup{Status: models.LocationGroupStatusActive}
	c.Assert(active.IsActive(), qt.IsTrue)

	pendingDeletion := models.LocationGroup{Status: models.LocationGroupStatusPendingDeletion}
	c.Assert(pendingDeletion.IsActive(), qt.IsFalse)
}

func TestGenerateGroupSlug(t *testing.T) {
	c := qt.New(t)

	slug1, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	c.Assert(len(slug1) >= 22, qt.IsTrue, qt.Commentf("slug should be at least 22 chars, got %d: %s", len(slug1), slug1))

	// Verify uniqueness (two calls produce different slugs)
	slug2, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	c.Assert(slug1, qt.Not(qt.Equals), slug2)
}

func TestLocationGroup_Validate_ReturnsError(t *testing.T) {
	c := qt.New(t)

	lg := &models.LocationGroup{}
	err := lg.Validate()
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.Equals, models.ErrMustUseValidateWithContext)
}
