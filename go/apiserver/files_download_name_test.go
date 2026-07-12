package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/blobkeys"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
)

// TestDownload_ContentDispositionFilename asserts the filename the user actually
// gets — the `Content-Disposition` header on the real download response (#2250).
//
// This is the boundary that decides. `files.path` is nominally the name without
// its extension, but the API accepts one that carries it, and the handler used to
// send `file.Path + file.Ext` — so a row with path="receipt.pdf", ext=".pdf" made
// the browser save `receipt.pdf.pdf`.
//
// A frontend fix could not repair that, and this is the lesson the test encodes:
// per RFC 6266 the header takes PRIORITY over an <a download> attribute, so a
// test that asserted the DOM attribute passed while the saved filename was
// unchanged. Assert where the outcome is decided, not where the value was set.
func TestDownload_ContentDispositionFilename(t *testing.T) {
	tests := []struct {
		name string
		path string
		ext  string
		want string
	}{
		{
			name: "path already carries the extension",
			path: "receipt.pdf",
			ext:  ".pdf",
			want: `attachment; filename=receipt.pdf`,
		},
		{
			name: "path without an extension gets one",
			path: "receipt",
			ext:  ".pdf",
			want: `attachment; filename=receipt.pdf`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			params, testUser, testGroup := newParams()
			handler := apiserver.APIServer(params, &mockRestoreWorker{hasRunningRestores: false})

			// Seed a file row plus its blob, exactly as an upload would leave them:
			// the key is UUID-minted, the human name lives on Path.
			ctx := appctx.WithUser(context.Background(), testUser)
			registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
			ctx = appctx.WithGroup(ctx, testGroup)

			blobKey := blobkeys.BuildFileBlobKey(testUser.TenantID, "f47ac10b-58cc-4372-a567-0e02b2c3d479", tt.ext)
			bucket := must.Must(blob.OpenBucket(ctx, params.UploadLocation))
			defer bucket.Close()
			c.Assert(bucket.WriteAll(ctx, blobKey, []byte("%PDF-1.4 fake"), nil), qt.IsNil)

			file := must.Must(registrySet.FileRegistry.Create(ctx, models.FileEntity{
				TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
					TenantID: testUser.TenantID, GroupID: testGroup.ID, CreatedByUserID: testUser.ID,
				},
				Title: "Receipt",
				Type:  models.FileTypeDocument,
				File: &models.File{
					Path: tt.path, OriginalPath: blobKey, Ext: tt.ext, MIMEType: "application/pdf",
				},
			}))

			// Ask the API for a signed URL, then follow it — the same two steps the
			// browser takes.
			req := httptest.NewRequest(http.MethodPost,
				"/api/v1/g/"+testGroup.Slug+"/files/"+file.ID+"/signed-url", bytes.NewReader(nil))
			addTestUserAuthHeader(req, testUser.ID)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))

			var signed jsonapi.SignedFileURLResponse
			c.Assert(json.Unmarshal(rr.Body.Bytes(), &signed), qt.IsNil)

			dl := httptest.NewRequest(http.MethodGet, signed.Attributes.URL, nil)
			dlRR := httptest.NewRecorder()
			handler.ServeHTTP(dlRR, dl)
			c.Assert(dlRR.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", dlRR.Body.String()))

			c.Assert(dlRR.Header().Get("Content-Disposition"), qt.Equals, tt.want)
		})
	}
}
