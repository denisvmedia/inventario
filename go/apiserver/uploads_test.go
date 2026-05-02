package apiserver_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
)

// Legacy `/uploads/{commodities,locations}/{id}/*` tests
// (`TestUploads`, `TestUploads_invalid_upload`) were removed under
// #1421 alongside the routes themselves. Clients now POST to
// `/uploads/file` and pass `linked_entity_*` in the FileEntity
// payload. `TestUploads_restores*` below stays — the restore upload
// endpoint is unrelated to the per-entity legacy surface.

func TestUploads_restores(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()

	// Create a buffer to write the form data
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)

	// Create a file field in the form
	h := CreateFormFileMIME("files", "test.xml", "application/xml")
	fileWriter, err := bodyWriter.CreatePart(h)
	c.Assert(err, qt.IsNil)

	// Write XML content to the file field
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<inventory xmlns="http://inventario.example.com/schema" exportDate="2023-01-01T00:00:00Z" exportType="full">
  <locations>
    <location id="test-location">
      <name>Test Location</name>
    </location>
  </locations>
</inventory>`
	_, err = fileWriter.Write([]byte(xmlContent))
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
	c.Assert(response.Attributes.FileNames[0], qt.Matches, `test-\d+\.xml`)
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
