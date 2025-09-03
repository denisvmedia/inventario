package postgres_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
)

func TestImageRegistry_Create_HappyPath(t *testing.T) {
	testCases := []struct {
		name  string
		image models.Image
	}{
		{
			name: "valid image with all fields",
			image: models.Image{
				File: &models.File{
					Path:         "test-image",
					OriginalPath: "test-image.jpg",
					Ext:          ".jpg",
					MIMEType:     "image/jpeg",
				},
				// Note: ID will be generated server-side for security
			},
		},
		{
			name: "valid image with different format",
			image: models.Image{
				File: &models.File{
					Path:         "another-image",
					OriginalPath: "another-image.png",
					Ext:          ".png",
					MIMEType:     "image/png",
				},
				// Note: ID will be generated server-side for security
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := appctx.WithUser(c.Context(), &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "test-user-id"},
					TenantID: "test-tenant-id",
				},
			})

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Registry is already user-aware from setupTestRegistrySet

			// Create test hierarchy
			location := createTestLocation(c, registrySet)
			area := createTestArea(c, registrySet, location.ID)
			commodity := createTestCommodity(c, registrySet, area.ID)

			// Set commodity ID
			tc.image.CommodityID = commodity.ID

			// Create image
			result, err := registrySet.ImageRegistry.Create(ctx, tc.image)
			c.Assert(err, qt.IsNil)
			c.Assert(result, qt.IsNotNil)
			c.Assert(result.ID, qt.Not(qt.Equals), "")
			c.Assert(result.CommodityID, qt.Equals, tc.image.CommodityID)
			c.Assert(result.File.Path, qt.Equals, tc.image.File.Path)
			c.Assert(result.File.OriginalPath, qt.Equals, tc.image.File.OriginalPath)
			c.Assert(result.File.Ext, qt.Equals, tc.image.File.Ext)
			c.Assert(result.File.MIMEType, qt.Equals, tc.image.File.MIMEType)
		})
	}
}

func TestImageRegistry_Create_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name  string
		image models.Image
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
		},
		{
			name: "non-existent commodity",
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
			name: "missing file",
			image: models.Image{
				CommodityID: "some-commodity-id",
			},
		},
		{
			name:  "empty image",
			image: models.Image{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := appctx.WithUser(c.Context(), &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "test-user-id"},
					TenantID: "test-tenant-id",
				},
			})

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Registry is already user-aware from setupTestRegistrySet

			// For valid commodity ID tests, create test hierarchy
			if tc.image.CommodityID != "" && tc.image.CommodityID != "non-existent-commodity" {
				location := createTestLocation(c, registrySet)
				area := createTestArea(c, registrySet, location.ID)
				commodity := createTestCommodity(c, registrySet, area.ID)
				tc.image.CommodityID = commodity.ID
			}

			// Attempt to create invalid image
			result, err := registrySet.ImageRegistry.Create(ctx, tc.image)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestImageRegistry_Get_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	// Create test hierarchy and image
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)
	created := createTestImage(c, registrySet, commodity.ID)

	// Get the image
	result, err := registrySet.ImageRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.ID, qt.Equals, created.ID)
	c.Assert(result.CommodityID, qt.Equals, created.CommodityID)
	c.Assert(result.File.Path, qt.Equals, created.File.Path)
	c.Assert(result.File.OriginalPath, qt.Equals, created.File.OriginalPath)
	c.Assert(result.File.Ext, qt.Equals, created.File.Ext)
	c.Assert(result.File.MIMEType, qt.Equals, created.File.MIMEType)
}

func TestImageRegistry_Get_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name string
		id   string
	}{
		{
			name: "non-existent image",
			id:   "non-existent-id",
		},
		{
			name: "empty ID",
			id:   "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := c.Context()

			result, err := registrySet.ImageRegistry.Get(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestImageRegistry_List_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	// Initially should be empty
	images, err := registrySet.ImageRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(images), qt.Equals, 0)

	// Create test hierarchy and images
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)
	image1 := createTestImage(c, registrySet, commodity.ID)
	image2 := createTestImage(c, registrySet, commodity.ID)

	// List should now contain both images
	images, err = registrySet.ImageRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(images), qt.Equals, 2)

	// Verify the images are correct
	imageIDs := make(map[string]bool)
	for _, image := range images {
		imageIDs[image.ID] = true
	}
	c.Assert(imageIDs[image1.ID], qt.IsTrue)
	c.Assert(imageIDs[image2.ID], qt.IsTrue)
}

func TestImageRegistry_Update_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	// Create test hierarchy and image
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)
	created := createTestImage(c, registrySet, commodity.ID)

	// Update the image
	created.File.Path = "updated-image-path"
	created.File.MIMEType = "image/png"

	result, err := registrySet.ImageRegistry.Update(ctx, *created)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.ID, qt.Equals, created.ID)
	c.Assert(result.File.Path, qt.Equals, "updated-image-path")
	c.Assert(result.File.MIMEType, qt.Equals, "image/png")

	// Verify the update persisted
	retrieved, err := registrySet.ImageRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved.File.Path, qt.Equals, "updated-image-path")
	c.Assert(retrieved.File.MIMEType, qt.Equals, "image/png")
}

func TestImageRegistry_Update_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name  string
		image models.Image
	}{
		{
			name: "non-existent image",
			image: models.Image{
				TenantAwareEntityID: models.WithTenantAwareEntityID("non-existent-id", "test-tenant-id"),
				CommodityID:         "some-commodity-id",
				File: &models.File{
					Path:         "test-image",
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
			ctx := appctx.WithUser(c.Context(), &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "test-user-id"},
					TenantID: "test-tenant-id",
				},
			})

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// For valid commodity ID tests, create test hierarchy
			if tc.image.CommodityID != "" && tc.image.CommodityID != "non-existent-commodity" {
				location := createTestLocation(c, registrySet)
				area := createTestArea(c, registrySet, location.ID)
				commodity := createTestCommodity(c, registrySet, area.ID)
				tc.image.CommodityID = commodity.ID
			}

			// Attempt to update non-existent image
			result, err := registrySet.ImageRegistry.Update(ctx, tc.image)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestImageRegistry_Delete_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	// Create test hierarchy and image
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)
	created := createTestImage(c, registrySet, commodity.ID)

	// Delete the image
	err := registrySet.ImageRegistry.Delete(ctx, created.ID)
	c.Assert(err, qt.IsNil)

	// Verify the image is deleted
	result, err := registrySet.ImageRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNotNil)
	c.Assert(result, qt.IsNil)
}

func TestImageRegistry_Delete_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name string
		id   string
	}{
		{
			name: "non-existent image",
			id:   "non-existent-id",
		},
		{
			name: "empty ID",
			id:   "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := c.Context()

			err := registrySet.ImageRegistry.Delete(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

func TestImageRegistry_Count_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	// Initially should be 0
	count, err := registrySet.ImageRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Create test hierarchy and images
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)
	createTestImage(c, registrySet, commodity.ID)
	createTestImage(c, registrySet, commodity.ID)

	// Count should now be 2
	count, err = registrySet.ImageRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}
