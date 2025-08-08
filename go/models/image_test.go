package models_test

import (
	"context"
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestImage_Validate(t *testing.T) {
	c := qt.New(t)

	image := &models.Image{}
	err := image.Validate()
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorIs, models.ErrMustUseValidateWithContext)
}

func TestImage_ValidateWithContext_HappyPath(t *testing.T) {
	t.Run("valid image", func(t *testing.T) {
		c := qt.New(t)

		image := models.Image{
			CommodityID: "commodity-123",
			File: &models.File{
				Path:         "test-image",
				OriginalPath: "test-image.jpg",
				Ext:          ".jpg",
				MIMEType:     "image/jpeg",
			},
		}

		ctx := context.Background()
		err := image.ValidateWithContext(ctx)
		c.Assert(err, qt.IsNil)
	})
}

func TestImage_ValidateWithContext_UnhappyPaths(t *testing.T) {
	testCases := []struct {
		name          string
		image         models.Image
		errorContains string
	}{
		{
			name: "missing commodity ID",
			image: models.Image{
				File: &models.File{
					Path:         "test-image",
					OriginalPath: "test-image.jpg",
					Ext:          ".jpg",
					MIMEType:     "image/jpeg",
				},
			},
			errorContains: "commodity_id: cannot be blank",
		},
		{
			name: "missing file",
			image: models.Image{
				CommodityID: "commodity-123",
			},
			errorContains: "File: cannot be blank",
		},
		{
			name:          "empty image",
			image:         models.Image{},
			errorContains: "File: cannot be blank",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			ctx := context.Background()
			err := tc.image.ValidateWithContext(ctx)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.errorContains)
		})
	}
}

func TestImage_JSONMarshaling(t *testing.T) {
	c := qt.New(t)

	// Create an image with all fields populated
	image := models.Image{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{
				ID: "image-123",
			},
			TenantID: "test-tenant",
		},
		CommodityID: "commodity-123",
		File: &models.File{
			Path:         "test-image",
			OriginalPath: "test-image.jpg",
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(image)
	c.Assert(err, qt.IsNil)

	// Unmarshal back to a new image
	var newImage models.Image
	err = json.Unmarshal(data, &newImage)
	c.Assert(err, qt.IsNil)

	// Verify fields match
	c.Assert(newImage.ID, qt.Equals, image.ID)
	c.Assert(newImage.CommodityID, qt.Equals, image.CommodityID)

	// Verify File fields
	c.Assert(newImage.File, qt.IsNotNil)
	c.Assert(newImage.File.Path, qt.Equals, image.File.Path)
	c.Assert(newImage.File.OriginalPath, qt.Equals, image.File.OriginalPath)
	c.Assert(newImage.File.Ext, qt.Equals, image.File.Ext)
	c.Assert(newImage.File.MIMEType, qt.Equals, image.File.MIMEType)
}

func TestImage_IDable(t *testing.T) {
	c := qt.New(t)

	// Create an image
	image := models.Image{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{
				ID: "image-123",
			},
			TenantID: "test-tenant",
		},
		CommodityID: "commodity-123",
		File: &models.File{
			Path:         "test-image",
			OriginalPath: "test-image.jpg",
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	}

	// Test GetID
	c.Assert(image.GetID(), qt.Equals, "image-123")

	// Test SetID
	image.SetID("new-image-id")
	c.Assert(image.GetID(), qt.Equals, "new-image-id")
}
