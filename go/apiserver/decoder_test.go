package apiserver_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/apiserver"
)

func TestGetRequestContentType(t *testing.T) {
	testCases := []struct {
		name           string
		header         string
		expectedResult render.ContentType
	}{
		{
			name:           "Empty header",
			header:         "",
			expectedResult: render.ContentTypeUnknown,
		},
		{
			name:           "Valid header",
			header:         "text/plain;charset=utf-8",
			expectedResult: render.ContentTypePlainText,
		},
		{
			name:           "Multiple headers",
			header:         "text/html; charset=utf-8",
			expectedResult: render.ContentTypeHTML,
		},
		{
			name:           "Unknown content type",
			header:         "application/custom",
			expectedResult: render.ContentTypeUnknown,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			r := &http.Request{Header: http.Header{"Content-Type": {tc.header}}}
			result := apiserver.GetRequestContentType(r)
			c.Assert(result, qt.Equals, tc.expectedResult)
		})
	}
}

func TestGetContentType(t *testing.T) {
	testCases := []struct {
		name           string
		contentType    string
		expectedResult render.ContentType
	}{
		{
			name:           "Plain text",
			contentType:    "text/plain",
			expectedResult: render.ContentTypePlainText,
		},
		{
			name:           "HTML",
			contentType:    "text/html",
			expectedResult: render.ContentTypeHTML,
		},
		{
			name:           "JSON",
			contentType:    "application/json",
			expectedResult: render.ContentTypeJSON,
		},
		{
			name:           "XML",
			contentType:    "text/xml",
			expectedResult: render.ContentTypeXML,
		},
		{
			name:           "Unknown content type",
			contentType:    "application/custom",
			expectedResult: render.ContentTypeUnknown,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			result := apiserver.GetContentType(tc.contentType)
			c.Assert(result, qt.Equals, tc.expectedResult)
		})
	}
}

func TestJSONAPIAwareDecoder(t *testing.T) {
	testCases := []struct {
		name            string
		contentType     render.ContentType
		contentTypeText string
		body            io.ReadCloser
		decodeTo        any
		expectedError   error
	}{
		{
			name:            "JSON content type",
			contentType:     render.ContentTypeJSON,
			contentTypeText: "application/json; charset=utf-8",
			body:            io.NopCloser(strings.NewReader("{}")),
			expectedError:   nil,
		},
		{
			name:            "XML content type",
			contentType:     render.ContentTypeXML,
			contentTypeText: "application/xml; charset=utf-8",
			body:            io.NopCloser(strings.NewReader("<!?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<root></root>")),
			expectedError:   nil,
		},
		{
			name:            "Form content type",
			contentType:     render.ContentTypeForm,
			contentTypeText: "application/x-www-form-urlencoded",
			body:            io.NopCloser(strings.NewReader("key=value")),
			decodeTo:        make(map[string]string),
			expectedError:   nil,
		},
		{
			name:          "Unknown content type",
			contentType:   render.ContentTypeUnknown,
			expectedError: apiserver.ErrUnknownContentType,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			r := &http.Request{
				Header: http.Header{
					"Content-Type": {tc.contentTypeText},
				},
				Body: tc.body,
			}
			err := apiserver.JSONAPIAwareDecoder(r, &tc.decodeTo)
			c.Assert(err, qt.Equals, tc.expectedError)
		})
	}
}
