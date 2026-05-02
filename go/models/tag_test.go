package models_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestTagColor_IsValid(t *testing.T) {
	c := qt.New(t)

	for _, color := range models.ValidTagColors {
		c.Run(string(color), func(c *qt.C) {
			c.Assert(color.IsValid(), qt.IsTrue)
		})
	}

	c.Run("rejects unknown color", func(c *qt.C) {
		c.Assert(models.TagColor("rainbow").IsValid(), qt.IsFalse)
	})
	c.Run("rejects empty color", func(c *qt.C) {
		c.Assert(models.TagColor("").IsValid(), qt.IsFalse)
	})
}

func TestTagColor_Validate(t *testing.T) {
	c := qt.New(t)

	c.Assert(models.TagColorAmber.Validate(), qt.IsNil)
	c.Assert(models.TagColor("rainbow").Validate(), qt.IsNotNil)
}

func TestNormalizeTagSlug(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		in  string
		out string
	}{
		{"kitchen", "kitchen"},
		{"Kitchen", "kitchen"},
		{"  kitchen  ", "kitchen"},
		{"front office", "front-office"},
		{"FRONT_OFFICE", "front-office"},
		{"front---office", "front-office"},
		{"front!@#office", "front-office"},
		{"-leading-and-trailing-", "leading-and-trailing"},
		{"", ""},
		{"   ", ""},
		{"###", ""},
		{"unicode-ücase", "unicode-ücase"}, // NormalizeTagSlug lowercases unicode but doesn't strip non-ASCII letters
	}
	for _, tt := range tests {
		c.Run(tt.in, func(c *qt.C) {
			c.Assert(models.NormalizeTagSlug(tt.in), qt.Equals, tt.out)
		})
	}
}

func TestIsValidTagSlug(t *testing.T) {
	c := qt.New(t)

	valid := []string{"kitchen", "front-office", "tag-1", "a", "a-1-2"}
	invalid := []string{"", "Kitchen", "front_office", "front--office", "-front", "front-", "front office"}

	for _, s := range valid {
		c.Run("valid:"+s, func(c *qt.C) {
			c.Assert(models.IsValidTagSlug(s), qt.IsTrue)
		})
	}
	for _, s := range invalid {
		c.Run("invalid:"+s, func(c *qt.C) {
			c.Assert(models.IsValidTagSlug(s), qt.IsFalse)
		})
	}
}

func TestTag_ValidateWithContext(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	tests := []struct {
		name    string
		tag     models.Tag
		wantErr bool
	}{
		{
			name: "valid tag",
			tag: models.Tag{
				Slug:  "kitchen",
				Label: "Kitchen",
				Color: models.TagColorAmber,
			},
			wantErr: false,
		},
		{
			name: "missing slug",
			tag: models.Tag{
				Label: "Kitchen",
				Color: models.TagColorAmber,
			},
			wantErr: true,
		},
		{
			name: "invalid slug shape",
			tag: models.Tag{
				Slug:  "Kitchen Office",
				Label: "Kitchen Office",
				Color: models.TagColorAmber,
			},
			wantErr: true,
		},
		{
			name: "missing label",
			tag: models.Tag{
				Slug:  "kitchen",
				Color: models.TagColorAmber,
			},
			wantErr: true,
		},
		{
			name: "label too long",
			tag: models.Tag{
				Slug:  "kitchen",
				Label: stringOfLen(65),
				Color: models.TagColorAmber,
			},
			wantErr: true,
		},
		{
			name: "missing color",
			tag: models.Tag{
				Slug:  "kitchen",
				Label: "Kitchen",
			},
			wantErr: true,
		},
		{
			name: "invalid color",
			tag: models.Tag{
				Slug:  "kitchen",
				Label: "Kitchen",
				Color: models.TagColor("rainbow"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			err := tt.tag.ValidateWithContext(ctx)
			if tt.wantErr {
				c.Assert(err, qt.IsNotNil)
			} else {
				c.Assert(err, qt.IsNil)
			}
		})
	}
}

func TestTag_TenantGroupAwareIDable(t *testing.T) {
	c := qt.New(t)

	var tag models.Tag
	var _ models.IDable = &tag
	var _ models.TenantAware = &tag
	var _ models.GroupAware = &tag
	var _ models.CreatedByUserAware = &tag
	var _ models.TenantGroupAwareIDable = &tag

	c.Assert(&tag, qt.IsNotNil)
}

func stringOfLen(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}
