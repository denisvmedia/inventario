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

func TestUploads(t *testing.T) {
	params := newParams()

	tcs := []struct {
		typ            string
		contentType    string
		filePath       string
		expectedLength func(c *qt.C) int
		checkResult    func(c *qt.C, expectedLen int, expectedCommodityID string)
	}{
		{
			typ:         "images",
			contentType: "image/jpeg",
			filePath:    "testdata/image.jpg",
			expectedLength: func(c *qt.C) int {
				images, err := params.RegistrySet.ImageRegistry.List(c.Context())
				c.Assert(err, qt.IsNil)
				expectedLen := len(images) + 1
				return expectedLen
			},
			checkResult: func(c *qt.C, expectedLen int, expectedCommodityID string) {
				images, err := params.RegistrySet.ImageRegistry.List(c.Context())
				c.Assert(err, qt.IsNil)
				c.Assert(images, qt.HasLen, expectedLen)
				c.Assert(images[expectedLen-1].Path, qt.Matches, `image-\d+`)
				c.Assert(images[expectedLen-1].Ext, qt.Equals, ".jpg")
				c.Assert(images[expectedLen-1].MIMEType, qt.Equals, "image/jpeg")
				c.Assert(images[expectedLen-1].CommodityID, qt.Equals, expectedCommodityID)
			},
		},
		{
			typ:         "manuals",
			contentType: "application/pdf",
			filePath:    "testdata/manual.pdf",
			expectedLength: func(c *qt.C) int {
				manuals, err := params.RegistrySet.ManualRegistry.List(c.Context())
				c.Assert(err, qt.IsNil)
				expectedLen := len(manuals) + 1
				return expectedLen
			},
			checkResult: func(c *qt.C, expectedLen int, expectedCommodityID string) {
				manuals, err := params.RegistrySet.ManualRegistry.List(c.Context())
				c.Assert(err, qt.IsNil)
				c.Assert(manuals, qt.HasLen, expectedLen)
				c.Assert(manuals[expectedLen-1].Path, qt.Matches, `manual-\d+`)
				c.Assert(manuals[expectedLen-1].Ext, qt.Equals, ".pdf")
				c.Assert(manuals[expectedLen-1].MIMEType, qt.Equals, "application/pdf")
				c.Assert(manuals[expectedLen-1].CommodityID, qt.Equals, expectedCommodityID)
			},
		},
		{
			typ:         "invoices",
			contentType: "application/pdf",
			filePath:    "testdata/invoice.pdf",
			expectedLength: func(c *qt.C) int {
				invoices, err := params.RegistrySet.InvoiceRegistry.List(c.Context())
				c.Assert(err, qt.IsNil)
				expectedLen := len(invoices) + 1
				return expectedLen
			},
			checkResult: func(c *qt.C, expectedLen int, expectedCommodityID string) {
				invoices, err := params.RegistrySet.InvoiceRegistry.List(c.Context())
				c.Assert(err, qt.IsNil)
				c.Assert(invoices, qt.HasLen, expectedLen)
				c.Assert(invoices[expectedLen-1].Path, qt.Matches, `invoice-\d+`)
				c.Assert(invoices[expectedLen-1].Ext, qt.Equals, ".pdf")
				c.Assert(invoices[expectedLen-1].MIMEType, qt.Equals, "application/pdf")
				c.Assert(invoices[expectedLen-1].CommodityID, qt.Equals, expectedCommodityID)
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.typ, func(t *testing.T) {
			c := qt.New(t)

			expectedCommodities := must.Must(params.RegistrySet.CommodityRegistry.List(c.Context()))
			commodity := expectedCommodities[0]
			expectedLen := tc.expectedLength(c)

			// Create a buffer to write the form data
			bodyBuf := &bytes.Buffer{}
			bodyWriter := multipart.NewWriter(bodyBuf)

			// Create a file field in the form
			h := CreateFormFileMIME("file", filepath.Base(tc.filePath), tc.contentType)
			fileWriter, err := bodyWriter.CreatePart(h)
			c.Assert(err, qt.IsNil)

			// Open the file and copy its contents to the file field
			file, err := os.Open(tc.filePath)
			c.Assert(err, qt.IsNil)
			defer file.Close()

			_, err = io.Copy(fileWriter, file)
			c.Assert(err, qt.IsNil)

			// Close the form writer
			contentType := bodyWriter.FormDataContentType()
			bodyWriter.Close()

			// Create a new request with the form data
			req, err := http.NewRequest("POST", "/api/v1/uploads/commodities/"+commodity.ID+"/"+tc.typ, bodyBuf)
			c.Assert(err, qt.IsNil)
			req.Header.Set("Content-Type", contentType)

			rr := httptest.NewRecorder()

			handler := apiserver.APIServer(params)
			handler.ServeHTTP(rr, req)

			// Verify the response
			c.Assert(rr.Code, qt.Equals, http.StatusCreated)

			// Verify the image is created in the registry
			tc.checkResult(c, expectedLen, commodity.ID)
		})
	}
}

func TestUploads_invalid_upload(t *testing.T) {
	tcs := []struct {
		typ         string
		contentType string
	}{
		{
			typ:         "images",
			contentType: "image/png",
		},
		{
			typ:         "manuals",
			contentType: "application/pdf",
		},
		{
			typ:         "invoices",
			contentType: "application/pdf",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.typ, func(t *testing.T) {
			c := qt.New(t)

			params := newParams()

			filePath := "testdata/invalid.txt"

			expectedCommodities := must.Must(params.RegistrySet.CommodityRegistry.List(c.Context()))
			commodity := expectedCommodities[0]

			// Create a buffer to write the form data
			bodyBuf := &bytes.Buffer{}
			bodyWriter := multipart.NewWriter(bodyBuf)

			// Create a file field in the form
			h := CreateFormFileMIME("file", filepath.Base(filePath), tc.contentType)
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
			req, err := http.NewRequest("POST", "/api/v1/uploads/commodities/"+commodity.ID+"/"+tc.typ, bodyBuf)
			c.Assert(err, qt.IsNil)
			req.Header.Set("Content-Type", contentType)

			rr := httptest.NewRecorder()

			handler := apiserver.APIServer(params)
			handler.ServeHTTP(rr, req)

			// Verify the response
			c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
		})
	}
}
