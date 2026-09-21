package apiserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

// downloadThumbnail interpolates `size` straight into the storage key, so the
// allowlist in front of it is the only thing between a URL segment and a
// bucket path. #2114 N2 had it at 0%.
//
// Exercised in-package like thumbnail_placeholder_test.go: the handler reads
// its parameters from the chi route context, so no signed URL and no blob are
// needed to reach the branches that matter.

// An in-memory bucket: ResolveThumbnail opens it to decide whether the
// thumbnail exists, so the placeholder branch is only reachable with one.
const thumbnailTestBucket = "file://thumbs?memfs=1&create_dir=1"

type thumbnailDownloadEnv struct {
	api    *filesAPI
	ctx    context.Context
	fileID string
}

func newThumbnailDownloadEnv(c *qt.C) *thumbnailDownloadEnv {
	c.Helper()

	fs := memory.NewFactorySet()

	const tenantID = "t1"
	const userID = "u1"
	const groupID = "g1"

	user := &models.User{TenantAwareEntityID: models.TenantAwareEntityID{
		TenantID: tenantID,
		EntityID: models.EntityID{ID: userID},
	}}
	group := &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenantID,
			EntityID: models.EntityID{ID: groupID},
		},
		GroupCurrency: models.Currency("USD"),
	}
	ctx := appctx.WithGroup(appctx.WithUser(context.Background(), user), group)

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

	return &thumbnailDownloadEnv{
		api: &filesAPI{
			factorySet:     fs,
			uploadLocation: thumbnailTestBucket,
			fileService:    services.NewFileService(fs, thumbnailTestBucket),
			thumbnailConfig: services.ThumbnailGenerationConfig{
				MaxConcurrentPerUser: 10,
				RateLimitPerMinute:   60,
				SlotDuration:         time.Minute,
			},
		},
		ctx:    ctx,
		fileID: created.ID,
	}
}

func (e *thumbnailDownloadEnv) get(c *qt.C, fileID, size string) *httptest.ResponseRecorder {
	c.Helper()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("fileID", fileID)
	rctx.URLParams.Add("size", size)

	req := httptest.NewRequest(http.MethodGet, "/files/download/thumbnails/x/y", nil).
		WithContext(context.WithValue(e.ctx, chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	e.api.downloadThumbnail(w, req)
	return w
}

// Anything outside the two known sizes is refused before the file is even
// looked up. `size` reaches the storage key, so a value that walks out of the
// namespace must never get that far.
func TestDownloadThumbnail_SizeIsAnAllowlist(t *testing.T) {
	refused := []struct {
		name string
		size string
	}{
		{"a size that does not exist", "large"},
		{"empty", ""},
		{"parent traversal", "../../etc/passwd"},
		{"a nested path", "small/../../secrets"},
		{"case does not count", "Small"},
		{"whitespace around a valid size", " small"},
	}

	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			env := newThumbnailDownloadEnv(c)

			w := env.get(c, env.fileID, tt.size)
			c.Assert(w.Code, qt.Equals, http.StatusNotFound,
				qt.Commentf("size %q was not refused", tt.size))
		})
	}
}

func TestDownloadThumbnail_MissingFileID(t *testing.T) {
	c := qt.New(t)
	env := newThumbnailDownloadEnv(c)

	w := env.get(c, "", "small")
	c.Assert(w.Code, qt.Equals, http.StatusNotFound)
}

// An id that resolves to nothing is a 404 from the registry, not a 500 —
// the file row is what decides which tenant's namespace to read.
func TestDownloadThumbnail_UnknownFileID(t *testing.T) {
	c := qt.New(t)
	env := newThumbnailDownloadEnv(c)

	w := env.get(c, "00000000-0000-0000-0000-000000000000", "small")
	c.Assert(w.Code, qt.Equals, http.StatusNotFound)
}

// The first view of an image has no thumbnail yet. That is the common case,
// and it must serve the placeholder and enqueue generation rather than fail.
func TestDownloadThumbnail_NoThumbnailYetServesThePlaceholder(t *testing.T) {
	for _, size := range []string{"small", "medium"} {
		t.Run(size, func(t *testing.T) {
			c := qt.New(t)
			env := newThumbnailDownloadEnv(c)

			w := env.get(c, env.fileID, size)
			c.Assert(w.Code, qt.Equals, http.StatusOK)
			c.Check(w.Header().Get("Content-Type"), qt.Equals, "image/gif")

			jobReg := env.api.factorySet.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry()
			job, err := jobReg.GetJobByFileID(env.ctx, env.fileID)
			c.Assert(err, qt.IsNil)
			c.Check(job.Status, qt.Equals, models.ThumbnailStatusPending)
		})
	}
}

// The refusal happens before the lookup, so a bad size on an unknown file is
// still a 404 and never reaches the registry.
func TestDownloadThumbnail_BadSizeIsRefusedBeforeTheLookup(t *testing.T) {
	c := qt.New(t)
	env := newThumbnailDownloadEnv(c)

	w := env.get(c, "00000000-0000-0000-0000-000000000000", "large")
	c.Assert(w.Code, qt.Equals, http.StatusNotFound)

	// Nothing was enqueued for a file that does not exist.
	jobReg := env.api.factorySet.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry()
	_, err := jobReg.GetJobByFileID(env.ctx, "00000000-0000-0000-0000-000000000000")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}
