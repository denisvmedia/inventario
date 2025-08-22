package apiserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
)

func TestDownloadWithOriginalPath(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	b, err := blob.OpenBucket(context.Background(), params.UploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()

	// Create a test file with a specific original path
	originalPath := "test-file-1234567890.pdf"
	err = b.WriteAll(context.Background(), originalPath, []byte("test content"), nil)
	c.Assert(err, qt.IsNil)

	// Create a manual with the original path
	commodity := must.Must(params.RegistrySet.CommodityRegistry.List(context.Background()))[0]
	manual := models.Manual{
		CommodityID: commodity.ID,
		File: &models.File{
			Path:         "test-file", // Just the filename without extension
			OriginalPath: originalPath,
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}
	createdManual := must.Must(params.RegistrySet.ManualRegistry.Create(context.Background(), manual))

	// Test downloading the file
	req, err := http.NewRequest("GET", "/api/v1/commodities/"+commodity.ID+"/manuals/"+createdManual.ID+".pdf", nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req)
	rr := httptest.NewRecorder()
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	// Verify the response
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Header().Get("Content-Type"), qt.Equals, "application/pdf")
	c.Assert(rr.Body.String(), qt.Equals, "test content")
}
