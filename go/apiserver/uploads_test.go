package apiserver_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/backupsign"
	"github.com/denisvmedia/inventario/internal/blobkeys"
	"github.com/denisvmedia/inventario/internal/inb"
)

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
