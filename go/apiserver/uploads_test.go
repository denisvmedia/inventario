package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/backupsign"
	"github.com/denisvmedia/inventario/internal/blobkeys"
	"github.com/denisvmedia/inventario/internal/inb"
)

// countBucketKeys returns the total number of blob keys in the bucket at
// uploadLocation. Used by the orphan-blob regression tests to assert that a
// rejected upload leaves the bucket exactly as it found it.
func countBucketKeys(c *qt.C, uploadLocation string) int {
	ctx := context.Background()
	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()

	count := 0
	iter := b.List(nil)
	for {
		obj, err := iter.Next(ctx)
		if err != nil {
			break
		}
		if obj.IsDir {
			continue
		}
		count++
	}
	return count
}

// tempUploadLocation returns a file:// bucket URL backed by a fresh temp dir so
// a test can enumerate every key without contaminating the package-shared
// memfs bucket.
func tempUploadLocation(c *qt.C) string {
	tempDir := c.TempDir()
	if runtime.GOOS == "windows" {
		return "file:///" + tempDir + "?create_dir=1"
	}
	return "file://" + tempDir + "?create_dir=1"
}

// buildMinimalINB builds a tiny, validly-framed `.inb` container for upload
// tests. The bytes are binary (sniffed as octet-stream, which the restore-upload
// guard accepts); the upload path never verifies the signature, so the payload
// content is irrelevant here.
func buildMinimalINB() []byte {
	seed := make([]byte, backupsign.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	signer, _ := backupsign.NewSigner(seed)
	payload := []byte("\x1f\x8b\x08minimal-gzip-like-bytes")
	digest := backupsign.NewDigest()
	_, _ = digest.Write(payload)
	sig := signer.SignDigest(digest.Sum(nil))

	var buf bytes.Buffer
	_ = inb.WriteContainer(&buf, sig, bytes.NewReader(payload), int64(len(payload)))
	return buf.Bytes()
}

// Legacy `/uploads/{commodities,locations}/{id}/*` tests
// (`TestUploads`, `TestUploads_invalid_upload`) were removed under
// #1421 alongside the routes themselves. Clients now POST a multipart
// file to `/uploads/file` (creates an unlinked FileEntity) and then
// PUT `/files/{id}` with `linked_entity_*` set to attach the row.
// `TestUploads_restores*` below stays — the restore upload endpoint
// is unrelated to the per-entity legacy surface.

func TestUploads_restores(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()

	// Create a buffer to write the form data
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)

	// Create a file field in the form. Restore uploads now accept signed `.inb`
	// archives (#534); the upload guard validates against INBContentTypes
	// (custom type + octet-stream), so we send a binary `.inb` body.
	h := CreateFormFileMIME("files", "backup.inb", "application/x-inventario-backup")
	fileWriter, err := bodyWriter.CreatePart(h)
	c.Assert(err, qt.IsNil)

	// Write a minimal binary `.inb` container body. The upload guard only sniffs
	// the content type (binary → octet-stream, accepted); signature verification
	// happens later at restore time, not at upload.
	inbContent := buildMinimalINB()
	_, err = fileWriter.Write(inbContent)
	c.Assert(err, qt.IsNil)

	// Close the form writer
	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()

	// Create a new request with the form data
	req, err := http.NewRequest("POST", "/api/v1/g/"+testGroup.Slug+"/uploads/restores", bodyBuf)
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", contentType)
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	// Verify the response
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	// Parse the response to verify the structure
	var response struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Type      string   `json:"type"`
			FileNames []string `json:"fileNames"`
		} `json:"attributes"`
	}

	err = json.Unmarshal(rr.Body.Bytes(), &response)
	c.Assert(err, qt.IsNil)
	c.Assert(response.Type, qt.Equals, "uploads")
	c.Assert(response.Attributes.Type, qt.Equals, "restores")
	c.Assert(response.Attributes.FileNames, qt.HasLen, 1)
	// Restore uploads land under the per-tenant `restores/` namespace
	// (#1793). The trailing segment is the filekit-sanitized basename of the
	// uploaded `.inb` archive (#534).
	c.Assert(response.Attributes.FileNames[0], qt.Matches, `t/[^/]+/restores/backup-\d+\.inb`)
}

func TestUploads_restores_invalid(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()

	// Create a buffer to write the form data
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)

	// Create a file field in the form with invalid content type
	h := CreateFormFileMIME("files", "test.txt", "text/plain")
	fileWriter, err := bodyWriter.CreatePart(h)
	c.Assert(err, qt.IsNil)

	// Write non-XML content to the file field
	_, err = fileWriter.Write([]byte("This is not XML content"))
	c.Assert(err, qt.IsNil)

	// Close the form writer
	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()

	// Create a new request with the form data
	req, err := http.NewRequest("POST", "/api/v1/g/"+testGroup.Slug+"/uploads/restores", bodyBuf)
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", contentType)
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	// Verify the response - should be rejected due to invalid content type
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}

// TestUploads_restores_invalid_noOrphanBlob is the #2125 regression: a restore
// upload whose body fails MIME validation (the restore endpoint only accepts
// signed `.inb` archives) must NOT leave a partially-written blob behind. The
// MIME reader returns the sniffed bytes alongside ErrInvalidContentType, so
// io.Copy has already streamed them into the writer; the deferred Close commits
// them. Without the saveFile cleanup that commit is an orphan with no owning
// file row. This test points the upload at a fresh temp bucket so it can
// enumerate every key and assert the rejected upload left the bucket empty.
func TestUploads_restores_invalid_noOrphanBlob(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	params.UploadLocation = tempUploadLocation(c)

	// A fresh temp bucket starts empty.
	c.Assert(countBucketKeys(c, params.UploadLocation), qt.Equals, 0)

	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)
	// text/plain is rejected by the restore endpoint's INB-only guard.
	h := CreateFormFileMIME("files", "not-a-backup.txt", "text/plain")
	fileWriter, err := bodyWriter.CreatePart(h)
	c.Assert(err, qt.IsNil)
	_, err = fileWriter.Write([]byte("this is plainly not a signed .inb backup archive"))
	c.Assert(err, qt.IsNil)
	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()

	req, err := http.NewRequest("POST", "/api/v1/g/"+testGroup.Slug+"/uploads/restores", bodyBuf)
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", contentType)
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))

	// The rejected upload left no orphan blob in the bucket.
	c.Assert(countBucketKeys(c, params.UploadLocation), qt.Equals, 0)
}

// TestUploads_file_tooLarge_noOrphanBlob is the #2125 + #2101 regression: a
// /uploads/file body that exceeds the per-file size cap is rejected with 413
// AND leaves no orphan blob. The oversize error surfaces mid-stream after the
// writer has already buffered/committed some bytes; the saveFile cleanup must
// remove them. A fresh temp bucket lets the test enumerate every key.
func TestUploads_file_tooLarge_noOrphanBlob(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	params.UploadLocation = tempUploadLocation(c)
	params.MaxUploadBytes = 16 // tiny cap so a small JPEG trips it

	c.Assert(countBucketKeys(c, params.UploadLocation), qt.Equals, 0)

	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for x := range 64 {
		for y := range 64 {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	var imgBuf bytes.Buffer
	c.Assert(jpeg.Encode(&imgBuf, img, &jpeg.Options{Quality: 90}), qt.IsNil)
	c.Assert(imgBuf.Len() > 16, qt.IsTrue)

	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)
	h := CreateFormFileMIME("file", "big-photo.jpg", "image/jpeg")
	fileWriter, err := bodyWriter.CreatePart(h)
	c.Assert(err, qt.IsNil)
	_, err = fileWriter.Write(imgBuf.Bytes())
	c.Assert(err, qt.IsNil)
	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()

	req, err := http.NewRequest("POST", "/api/v1/g/"+testGroup.Slug+"/uploads/file", bodyBuf)
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", contentType)
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusRequestEntityTooLarge, qt.Commentf("body=%s", rr.Body.String()))

	// The rejected (oversize) upload left no orphan blob.
	c.Assert(countBucketKeys(c, params.UploadLocation), qt.Equals, 0)
}

// TestUploads_file_tenantPrefixedKey is the #1793 regression test: a
// POST /uploads/file lands a FileEntity row whose OriginalPath is the
// tenant-prefixed `t/<tenant>/files/<basename>` blob key, never a flat
// legacy shape. Combined with the structural test on the helper
// (`TestKeysAlwaysCarryTenantNamespace`), this proves that the upload
// path physically cannot place a blob outside the authenticated
// tenant's namespace.
func TestUploads_file_tenantPrefixedKey(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()

	// Build a tiny JPEG payload so the MIME sniffer accepts the part.
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for x := range 4 {
		for y := range 4 {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var imgBuf bytes.Buffer
	c.Assert(jpeg.Encode(&imgBuf, img, &jpeg.Options{Quality: 80}), qt.IsNil)

	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)
	h := CreateFormFileMIME("file", "my-photo.jpg", "image/jpeg")
	fileWriter, err := bodyWriter.CreatePart(h)
	c.Assert(err, qt.IsNil)
	_, err = fileWriter.Write(imgBuf.Bytes())
	c.Assert(err, qt.IsNil)
	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()

	req, err := http.NewRequest("POST", "/api/v1/g/"+testGroup.Slug+"/uploads/file", bodyBuf)
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", contentType)
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))

	var resp struct {
		ID         string `json:"id"`
		Attributes struct {
			OriginalPath string `json:"original_path"`
			Path         string `json:"path"`
			Ext          string `json:"ext"`
		} `json:"attributes"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &resp), qt.IsNil)

	// OriginalPath is the canonical tenant-prefixed key; Path is the
	// human-readable basename (the Title is derived from it on the
	// handler side).
	c.Assert(blobkeys.HasTenantPrefix(resp.Attributes.OriginalPath), qt.IsTrue,
		qt.Commentf("OriginalPath %q must be tenant-prefixed", resp.Attributes.OriginalPath))
	c.Assert(strings.HasPrefix(resp.Attributes.OriginalPath, "t/"+testUser.TenantID+"/files/"), qt.IsTrue,
		qt.Commentf("OriginalPath %q must live under the authenticated tenant's namespace", resp.Attributes.OriginalPath))
	c.Assert(resp.Attributes.Path, qt.Not(qt.Contains), "t/")
}

// TestUploads_file_tooLarge is the #2101 regression test: a POST
// /uploads/file whose body exceeds the configured per-file size cap is
// rejected with 413 and persists no FileEntity row. Without the cap the
// streaming io.Copy is unbounded and one user can fill the shared blob
// store; the multipart path also bypasses the 1 MiB request-body cap in
// security_middleware.go, so the limit must live in the save path itself.
func TestUploads_file_tooLarge(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	// Tiny cap so a small payload trips it. 0/negative would disable the
	// cap entirely (the back-compat opt-out), so we set a positive value.
	params.MaxUploadBytes = 16

	// Count the seeded file rows up-front so the assertion proves the
	// rejected upload added nothing, regardless of the harness fixtures.
	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
	before := must.Must(registrySet.FileRegistry.Count(ctx))

	// Build a JPEG larger than the cap so the MIME sniff passes but the
	// total streamed bytes exceed MaxUploadBytes.
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for x := range 64 {
		for y := range 64 {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	var imgBuf bytes.Buffer
	c.Assert(jpeg.Encode(&imgBuf, img, &jpeg.Options{Quality: 90}), qt.IsNil)
	c.Assert(imgBuf.Len() > 16, qt.IsTrue, qt.Commentf("payload %d bytes must exceed the 16-byte cap", imgBuf.Len()))

	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)
	h := CreateFormFileMIME("file", "big-photo.jpg", "image/jpeg")
	fileWriter, err := bodyWriter.CreatePart(h)
	c.Assert(err, qt.IsNil)
	_, err = fileWriter.Write(imgBuf.Bytes())
	c.Assert(err, qt.IsNil)
	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()

	req, err := http.NewRequest("POST", "/api/v1/g/"+testGroup.Slug+"/uploads/file", bodyBuf)
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", contentType)
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusRequestEntityTooLarge, qt.Commentf("body=%s", rr.Body.String()))

	// No FileEntity row was persisted for the rejected upload.
	after := must.Must(registrySet.FileRegistry.Count(ctx))
	c.Assert(after, qt.Equals, before)
}

// TestUploads_file_capDisabled proves the #2101 opt-out: with
// MaxUploadBytes <= 0 a large file is accepted (201), so the cap is purely
// additive and existing deployments that don't set the knob keep working.
func TestUploads_file_capDisabled(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	params.MaxUploadBytes = 0 // disabled

	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for x := range 64 {
		for y := range 64 {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 200, A: 255})
		}
	}
	var imgBuf bytes.Buffer
	c.Assert(jpeg.Encode(&imgBuf, img, &jpeg.Options{Quality: 90}), qt.IsNil)

	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)
	h := CreateFormFileMIME("file", "ok-photo.jpg", "image/jpeg")
	fileWriter, err := bodyWriter.CreatePart(h)
	c.Assert(err, qt.IsNil)
	_, err = fileWriter.Write(imgBuf.Bytes())
	c.Assert(err, qt.IsNil)
	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()

	req, err := http.NewRequest("POST", "/api/v1/g/"+testGroup.Slug+"/uploads/file", bodyBuf)
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", contentType)
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
}
