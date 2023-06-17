package apiserver_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/apiserver"
)

func TestUploads_HandleImagesUpload(t *testing.T) {
	c := qt.New(t)

	params := newParams()

	filePath := "testdata/image.jpg"

	expectedCommodities := must.Must(params.CommodityRegistry.List())
	commodity := expectedCommodities[0]

	images, err := params.ImageRegistry.List()
	c.Assert(err, qt.IsNil)
	expectedLen := len(images) + 1

	// Create a buffer to write the form data
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)

	// Create a file field in the form
	h := CreateFormFileMIME("file", filepath.Base(filePath), "image/jpeg")
	fileWriter, err := bodyWriter.CreatePart(h)
	c.Assert(err, qt.IsNil)

	// Open the file and copy its contents to the file field
	file, err := os.Open(filePath)
	c.Assert(err, qt.IsNil)
	defer file.Close()

	_, err = io.Copy(fileWriter, file)
	c.Assert(err, qt.IsNil)

	// Close the form writer
	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()

	// Create a new request with the form data
	req, err := http.NewRequest("POST", "/api/v1/uploads/commodities/"+commodity.ID+"/images", bodyBuf)
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", contentType)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	// Verify the response
	c.Assert(rr.Code, qt.Equals, http.StatusCreated)

	// Verify the image is created in the registry
	images, err = params.ImageRegistry.List()
	c.Assert(err, qt.IsNil)
	c.Assert(images, qt.HasLen, expectedLen)
	c.Assert(images[expectedLen-1].Path, qt.Not(qt.Equals), "")
	c.Assert(images[expectedLen-1].CommodityID, qt.Equals, commodity.ID)
}

func TestUploads_HandleManualsUpload(t *testing.T) {
	c := qt.New(t)

	params := newParams()

	filePath := "testdata/manual.pdf"

	expectedCommodities := must.Must(params.CommodityRegistry.List())
	commodity := expectedCommodities[0]

	manuals, err := params.ManualRegistry.List()
	c.Assert(err, qt.IsNil)
	expectedLen := len(manuals) + 1

	// Create a buffer to write the form data
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)

	// Create a file field in the form
	h := CreateFormFileMIME("file", filepath.Base(filePath), "application/pdf")
	fileWriter, err := bodyWriter.CreatePart(h)
	c.Assert(err, qt.IsNil)

	// Open the file and copy its contents to the file field
	file, err := os.Open(filePath)
	c.Assert(err, qt.IsNil)
	defer file.Close()

	_, err = io.Copy(fileWriter, file)
	c.Assert(err, qt.IsNil)

	// Close the form writer
	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()

	// Create a new request with the form data
	req, err := http.NewRequest("POST", "/api/v1/uploads/commodities/"+commodity.ID+"/manuals", bodyBuf)
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", contentType)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	// Verify the response
	c.Assert(rr.Code, qt.Equals, http.StatusCreated)

	// Verify the manual is created in the registry
	manuals, err = params.ManualRegistry.List()
	c.Assert(err, qt.IsNil)
	c.Assert(manuals, qt.HasLen, expectedLen)
	c.Assert(manuals[expectedLen-1].Path, qt.Not(qt.Equals), "")
	c.Assert(manuals[expectedLen-1].CommodityID, qt.Equals, commodity.ID)
}

func TestUploads_HandleInvoicesUpload(t *testing.T) {
	c := qt.New(t)

	params := newParams()

	filePath := "testdata/invoice.pdf"

	expectedCommodities := must.Must(params.CommodityRegistry.List())
	commodity := expectedCommodities[0]

	invoices, err := params.InvoiceRegistry.List()
	c.Assert(err, qt.IsNil)
	expectedLen := len(invoices) + 1

	// Create a buffer to write the form data
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)

	// Create a file field in the form
	h := CreateFormFileMIME("file", filepath.Base(filePath), "application/pdf")
	fileWriter, err := bodyWriter.CreatePart(h)
	c.Assert(err, qt.IsNil)

	// Open the file and copy its contents to the file field
	file, err := os.Open(filePath)
	c.Assert(err, qt.IsNil)
	defer file.Close()

	_, err = io.Copy(fileWriter, file)
	c.Assert(err, qt.IsNil)

	// Close the form writer
	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()

	// Create a new request with the form data
	req, err := http.NewRequest("POST", "/api/v1/uploads/commodities/"+commodity.ID+"/invoices", bodyBuf)
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", contentType)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	// Verify the response
	c.Assert(rr.Code, qt.Equals, http.StatusCreated)

	// Verify the invoice is created in the registry
	invoices, err = params.InvoiceRegistry.List()
	c.Assert(err, qt.IsNil)
	c.Assert(invoices, qt.HasLen, expectedLen)
	c.Assert(invoices[expectedLen-1].Path, qt.Not(qt.Equals), "")
	c.Assert(invoices[expectedLen-1].CommodityID, qt.Equals, commodity.ID)
}
