package postgresql_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

// TestImageRegistry_Create_HappyPath tests successful image creation scenarios.
func TestImageRegistry_Create_HappyPath(t *testing.T) {
	testCases := []struct {
		name  string
		image models.Image
	}{
		{
			name: "basic image",
			image: models.Image{
				File: &models.File{
					Path:         "test-image",
					OriginalPath: "test-image.jpg",
					Ext:          ".jpg",
					MIMEType:     "image/jpeg",
				},
			},
		},
		{
			name: "PNG image",
			image: models.Image{
				File: &models.File{
					Path:         "screenshot",
					OriginalPath: "screenshot.png",
					Ext:          ".png",
					MIMEType:     "image/png",
				},
			},
		},
		{
			name: "image with special characters",
			image: models.Image{
				File: &models.File{
					Path:         "café-photo",
					OriginalPath: "café photo.jpg",
					Ext:          ".jpg",
					MIMEType:     "image/jpeg",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Create test hierarchy
			location := createTestLocation(c, registrySet.LocationRegistry)
			area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
			commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())
			tc.image.CommodityID = commodity.GetID()

			// Create image
			createdImage, err := registrySet.ImageRegistry.Create(ctx, tc.image)
			c.Assert(err, qt.IsNil)
			c.Assert(createdImage, qt.IsNotNil)
			c.Assert(createdImage.GetID(), qt.Not(qt.Equals), "")
			c.Assert(createdImage.CommodityID, qt.Equals, tc.image.CommodityID)
			c.Assert(createdImage.File.Path, qt.Equals, tc.image.File.Path)
			c.Assert(createdImage.File.OriginalPath, qt.Equals, tc.image.File.OriginalPath)
			c.Assert(createdImage.File.Ext, qt.Equals, tc.image.File.Ext)
			c.Assert(createdImage.File.MIMEType, qt.Equals, tc.image.File.MIMEType)

			// Verify count
			count, err := registrySet.ImageRegistry.Count(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(count, qt.Equals, 1)
		})
	}
}

// TestImageRegistry_Create_UnhappyPath tests image creation error scenarios.
func TestImageRegistry_Create_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name  string
		image models.Image
	}{
		{
			name: "empty commodity ID",
			image: models.Image{
				CommodityID: "",
				File: &models.File{
					Path:         "test-image",
					OriginalPath: "test-image.jpg",
					Ext:          ".jpg",
					MIMEType:     "image/jpeg",
				},
			},
		},
		{
			name: "non-existent commodity ID",
			image: models.Image{
				CommodityID: "non-existent-commodity",
				File: &models.File{
					Path:         "test-image",
					OriginalPath: "test-image.jpg",
					Ext:          ".jpg",
					MIMEType:     "image/jpeg",
				},
			},
		},
		{
			name: "nil file",
			image: models.Image{
				CommodityID: "some-commodity-id",
				File:        nil,
			},
		},
		{
			name: "empty path",
			image: models.Image{
				CommodityID: "some-commodity-id",
				File: &models.File{
					Path:         "",
					OriginalPath: "test-image.jpg",
					Ext:          ".jpg",
					MIMEType:     "image/jpeg",
				},
			},
		},
		{
			name: "empty original path",
			image: models.Image{
				CommodityID: "some-commodity-id",
				File: &models.File{
					Path:         "test-image",
					OriginalPath: "",
					Ext:          ".jpg",
					MIMEType:     "image/jpeg",
				},
			},
		},
		{
			name: "empty extension",
			image: models.Image{
				CommodityID: "some-commodity-id",
				File: &models.File{
					Path:         "test-image",
					OriginalPath: "test-image.jpg",
					Ext:          "",
					MIMEType:     "image/jpeg",
				},
			},
		},
		{
			name: "empty MIME type",
			image: models.Image{
				CommodityID: "some-commodity-id",
				File: &models.File{
					Path:         "test-image",
					OriginalPath: "test-image.jpg",
					Ext:          ".jpg",
					MIMEType:     "",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// For valid commodity ID tests, create test hierarchy
			if tc.image.CommodityID != "" && tc.image.CommodityID != "non-existent-commodity" {
				location := createTestLocation(c, registrySet.LocationRegistry)
				area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
				commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())
				tc.image.CommodityID = commodity.GetID()
			}

			// Attempt to create invalid image
			_, err := registrySet.ImageRegistry.Create(ctx, tc.image)
			c.Assert(err, qt.IsNotNil)

			// Verify count remains zero
			count, err := registrySet.ImageRegistry.Count(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(count, qt.Equals, 0)
		})
	}
}

// TestImageRegistry_Get_HappyPath tests successful image retrieval scenarios.
func TestImageRegistry_Get_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())
	image := createTestImage(c, registrySet.ImageRegistry, commodity.GetID())

	// Get the image
	retrievedImage, err := registrySet.ImageRegistry.Get(ctx, image.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedImage, qt.IsNotNil)
	c.Assert(retrievedImage.GetID(), qt.Equals, image.GetID())
	c.Assert(retrievedImage.CommodityID, qt.Equals, image.CommodityID)
	c.Assert(retrievedImage.File.Path, qt.Equals, image.File.Path)
	c.Assert(retrievedImage.File.OriginalPath, qt.Equals, image.File.OriginalPath)
	c.Assert(retrievedImage.File.Ext, qt.Equals, image.File.Ext)
	c.Assert(retrievedImage.File.MIMEType, qt.Equals, image.File.MIMEType)
}

// TestImageRegistry_Get_UnhappyPath tests image retrieval error scenarios.
func TestImageRegistry_Get_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name string
		id   string
	}{
		{
			name: "non-existent ID",
			id:   "non-existent-id",
		},
		{
			name: "empty ID",
			id:   "",
		},
		{
			name: "UUID format but non-existent",
			id:   "550e8400-e29b-41d4-a716-446655440000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Try to get non-existent image
			_, err := registrySet.ImageRegistry.Get(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err, qt.ErrorMatches, ".*not found.*")
		})
	}
}

// TestImageRegistry_Update_HappyPath tests successful image update scenarios.
func TestImageRegistry_Update_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())
	image := createTestImage(c, registrySet.ImageRegistry, commodity.GetID())

	// Update the image
	image.File.Path = "updated-image"

	updatedImage, err := registrySet.ImageRegistry.Update(ctx, *image)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedImage, qt.IsNotNil)
	c.Assert(updatedImage.GetID(), qt.Equals, image.GetID())
	c.Assert(updatedImage.File.Path, qt.Equals, "updated-image")

	// Verify the update persisted
	retrievedImage, err := registrySet.ImageRegistry.Get(ctx, image.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedImage.File.Path, qt.Equals, "updated-image")
}

// TestImageRegistry_Update_UnhappyPath tests image update error scenarios.
func TestImageRegistry_Update_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name  string
		image models.Image
	}{
		{
			name: "non-existent image",
			image: models.Image{
				EntityID:    models.EntityID{ID: "non-existent-id"},
				CommodityID: "some-commodity-id",
				File: &models.File{
					Path:         "test-image",
					OriginalPath: "test-image.jpg",
					Ext:          ".jpg",
					MIMEType:     "image/jpeg",
				},
			},
		},
		{
			name: "empty path",
			image: models.Image{
				EntityID:    models.EntityID{ID: "some-id"},
				CommodityID: "some-commodity-id",
				File: &models.File{
					Path:         "",
					OriginalPath: "test-image.jpg",
					Ext:          ".jpg",
					MIMEType:     "image/jpeg",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Try to update with invalid data
			_, err := registrySet.ImageRegistry.Update(ctx, tc.image)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

// TestImageRegistry_Delete_HappyPath tests successful image deletion scenarios.
func TestImageRegistry_Delete_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())
	image := createTestImage(c, registrySet.ImageRegistry, commodity.GetID())

	// Verify image exists
	_, err := registrySet.ImageRegistry.Get(ctx, image.GetID())
	c.Assert(err, qt.IsNil)

	// Delete the image
	err = registrySet.ImageRegistry.Delete(ctx, image.GetID())
	c.Assert(err, qt.IsNil)

	// Verify image is deleted
	_, err = registrySet.ImageRegistry.Get(ctx, image.GetID())
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorMatches, ".*not found.*")

	// Verify count is zero
	count, err := registrySet.ImageRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

// TestImageRegistry_Delete_UnhappyPath tests image deletion error scenarios.
func TestImageRegistry_Delete_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name string
		id   string
	}{
		{
			name: "non-existent ID",
			id:   "non-existent-id",
		},
		{
			name: "empty ID",
			id:   "",
		},
		{
			name: "UUID format but non-existent",
			id:   "550e8400-e29b-41d4-a716-446655440000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Try to delete non-existent image
			err := registrySet.ImageRegistry.Delete(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

// TestImageRegistry_List_HappyPath tests successful image listing scenarios.
func TestImageRegistry_List_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Test empty list
	images, err := registrySet.ImageRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(images, qt.HasLen, 0)

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())
	image1 := createTestImage(c, registrySet.ImageRegistry, commodity.GetID())

	image2 := models.Image{
		CommodityID: commodity.GetID(),
		File: &models.File{
			Path:         "second-image",
			OriginalPath: "second-image.png",
			Ext:          ".png",
			MIMEType:     "image/png",
		},
	}
	createdImage2, err := registrySet.ImageRegistry.Create(ctx, image2)
	c.Assert(err, qt.IsNil)

	// List all images
	images, err = registrySet.ImageRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(images, qt.HasLen, 2)

	// Verify images are in the list
	imageIDs := make(map[string]bool)
	for _, image := range images {
		imageIDs[image.GetID()] = true
	}
	c.Assert(imageIDs[image1.GetID()], qt.IsTrue)
	c.Assert(imageIDs[createdImage2.GetID()], qt.IsTrue)
}

// TestImageRegistry_Count_HappyPath tests successful image counting scenarios.
func TestImageRegistry_Count_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Test empty count
	count, err := registrySet.ImageRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())
	createTestImage(c, registrySet.ImageRegistry, commodity.GetID())

	count, err = registrySet.ImageRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)

	// Create another image
	image2 := models.Image{
		CommodityID: commodity.GetID(),
		File: &models.File{
			Path:         "second-image",
			OriginalPath: "second-image.png",
			Ext:          ".png",
			MIMEType:     "image/png",
		},
	}
	_, err = registrySet.ImageRegistry.Create(ctx, image2)
	c.Assert(err, qt.IsNil)

	count, err = registrySet.ImageRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}

// TestImageRegistry_CascadeDelete tests that deleting a commodity cascades to images.
func TestImageRegistry_CascadeDelete(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())
	image := createTestImage(c, registrySet.ImageRegistry, commodity.GetID())

	// Verify image exists
	_, err := registrySet.ImageRegistry.Get(ctx, image.GetID())
	c.Assert(err, qt.IsNil)

	// Delete the commodity (should cascade to image)
	err = registrySet.CommodityRegistry.Delete(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)

	// Verify image is also deleted due to cascade
	_, err = registrySet.ImageRegistry.Get(ctx, image.GetID())
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorMatches, ".*not found.*")

	// Verify count is zero
	count, err := registrySet.ImageRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}