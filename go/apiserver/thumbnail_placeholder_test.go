package apiserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// TestServePlaceholderThumbnail_NoJobEnqueuesAndServesPlaceholder pins the
// fix for the thumbnail download 500: when a file has no thumbnail and no
// generation job yet (the first-view case, and always true for seeded
// fixtures), the handler must serve the placeholder (HTTP 200) and enqueue
// a generation job — NOT 500.
//
// The regression it guards: GetJobByFileID returns registry.ErrNotFound
// wrapped, but the handler used to match apiserver.ErrNotFound. errors.Is
// doesn't match in that direction (apiserver.ErrNotFound *wraps*
// registry.ErrNotFound, not the reverse), so the not-found landed in the
// 500 branch and on-demand generation never fired.
func TestServePlaceholderThumbnail_NoJobEnqueuesAndServesPlaceholder(t *testing.T) {
	c := qt.New(t)
	fs := memory.NewFactorySet()

	const tenantID = "t1"
	const userID = "u1"
	const groupID = "g1"

	user := &models.User{TenantAwareEntityID: models.TenantAwareEntityID{
		TenantID: tenantID,
		EntityID: models.EntityID{ID: userID},
	}}
	group := &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID, EntityID: models.EntityID{ID: groupID}},
		GroupCurrency:       models.Currency("USD"),
	}
	ctx := appctx.WithGroup(appctx.WithUser(context.Background(), user), group)

	// An image file row with no thumbnail and no generation job — exactly
	// the state a freshly seeded fixture image is in.
	created, err := fs.FileRegistryFactory.CreateServiceRegistry().Create(ctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        tenantID,
			GroupID:         groupID,
			CreatedByUserID: userID,
		},
		Title:    "Cover",
		Type:     models.FileTypeImage,
		Category: models.FileCategoryImages,
		File: &models.File{
			Path:         "photo-livingroom",
			OriginalPath: "t/t1/seed-abc.jpg",
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
			SizeBytes:    1,
		},
	})
	c.Assert(err, qt.IsNil)

	api := &filesAPI{
		factorySet:     fs,
		uploadLocation: "", // no bucket needed: we exercise the placeholder + enqueue path only
		thumbnailConfig: services.ThumbnailGenerationConfig{
			MaxConcurrentPerUser: 10,
			RateLimitPerMinute:   60,
			SlotDuration:         time.Minute,
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/files/download/thumbnails/"+created.ID+"/small", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	api.servePlaceholderThumbnail(w, req, created.ID, "small")

	c.Assert(w.Code, qt.Equals, http.StatusOK)
	c.Assert(w.Header().Get("Content-Type"), qt.Equals, "image/gif")

	// The placeholder response must come with an enqueued pending job so a
	// worker actually generates the thumbnail.
	job, err := fs.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry().GetJobByFileID(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(job, qt.IsNotNil)
	c.Assert(job.Status, qt.Equals, models.ThumbnailStatusPending)
}
