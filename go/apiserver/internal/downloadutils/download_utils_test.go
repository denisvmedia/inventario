package downloadutils_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver/internal/downloadutils"
)

func TestGetFileAttributes(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Test with invalid bucket URL (should return error)
	attrs, err := downloadutils.GetFileAttributes(ctx, "mem://", "missing.txt")
	c.Assert(err, qt.IsNotNil)
	c.Assert(attrs, qt.IsNil)
	c.Assert(err.Error(), qt.Contains, "failed to open bucket")
}

func TestGetFileAttributes_InvalidBucket(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Test with invalid bucket URL
	attrs, err := downloadutils.GetFileAttributes(ctx, "invalid://bucket", "test.txt")

	c.Assert(err, qt.IsNotNil)
	c.Assert(attrs, qt.IsNil)
	c.Assert(err.Error(), qt.Contains, "failed to open bucket")
}

func TestCopyFileInChunks(t *testing.T) {
	tests := []struct {
		name         string
		inputData    []byte
		expectError  bool
		expectOutput []byte
	}{
		{
			name:         "small file",
			inputData:    []byte("hello world"),
			expectError:  false,
			expectOutput: []byte("hello world"),
		},
		{
			name:         "empty file",
			inputData:    []byte{},
			expectError:  false,
			expectOutput: []byte{},
		},
		{
			name:         "large file",
			inputData:    bytes.Repeat([]byte("test data "), 10000), // ~100KB
			expectError:  false,
			expectOutput: bytes.Repeat([]byte("test data "), 10000),
		},
		{
			name:         "file larger than chunk size",
			inputData:    make([]byte, 64*1024), // 64KB (larger than 32KB chunk)
			expectError:  false,
			expectOutput: make([]byte, 64*1024),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Create reader from input data
			reader := bytes.NewReader(tt.inputData)

			// Create response recorder
			rr := httptest.NewRecorder()

			// Test CopyFileInChunks
			err := downloadutils.CopyFileInChunks(rr, reader)

			if tt.expectError {
				c.Assert(err, qt.IsNotNil)
			} else {
				c.Assert(err, qt.IsNil)
				// For empty files, check if the body is empty rather than comparing exact byte slices
				if len(tt.expectOutput) == 0 {
					c.Assert(rr.Body.Len(), qt.Equals, 0)
				} else {
					c.Assert(rr.Body.Bytes(), qt.DeepEquals, tt.expectOutput)
				}
			}
		})
	}
}

// errorReader is a test helper that returns an error after reading some data
type errorReader struct {
	data      []byte
	readCount int
	errorAt   int
}

func (er *errorReader) Read(p []byte) (n int, err error) {
	if er.readCount >= er.errorAt {
		return 0, errors.New("simulated read error")
	}

	if er.readCount >= len(er.data) {
		return 0, io.EOF
	}

	n = copy(p, er.data[er.readCount:])
	er.readCount += n
	return n, nil
}

func TestCopyFileInChunks_ReadError(t *testing.T) {
	c := qt.New(t)

	// Create reader that will error after reading some data
	reader := &errorReader{
		data:    []byte("hello world"),
		errorAt: 5, // Error after reading 5 bytes
	}

	rr := httptest.NewRecorder()

	err := downloadutils.CopyFileInChunks(rr, reader)

	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "failed to read chunk from file")
	c.Assert(err.Error(), qt.Contains, "simulated read error")
}

// errorWriter is a test helper that returns an error when writing
type errorWriter struct {
	*httptest.ResponseRecorder
	errorOnWrite bool
}

func (ew *errorWriter) Write(p []byte) (n int, err error) {
	if ew.errorOnWrite {
		return 0, errors.New("simulated write error")
	}
	return ew.ResponseRecorder.Write(p)
}

func (ew *errorWriter) Header() http.Header {
	return ew.ResponseRecorder.Header()
}

func (ew *errorWriter) WriteHeader(statusCode int) {
	ew.ResponseRecorder.WriteHeader(statusCode)
}

func TestCopyFileInChunks_WriteError(t *testing.T) {
	c := qt.New(t)

	reader := strings.NewReader("hello world")
	writer := &errorWriter{
		ResponseRecorder: httptest.NewRecorder(),
		errorOnWrite:     true,
	}

	err := downloadutils.CopyFileInChunks(writer, reader)

	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "failed to write chunk to response")
	c.Assert(err.Error(), qt.Contains, "simulated write error")
}

func TestSetStreamingHeaders(t *testing.T) {
	tests := []struct {
		name          string
		contentType   string
		fileSize      int64
		filename      string
		expectHeaders map[string]string
	}{
		{
			name:        "basic headers with filename",
			contentType: "application/xml",
			fileSize:    1024,
			filename:    "export.xml",
			expectHeaders: map[string]string{
				"Content-Type":        "application/xml",
				"Content-Length":      "1024",
				"Cache-Control":       "no-cache, no-store, must-revalidate",
				"Pragma":              "no-cache",
				"Expires":             "0",
				"Accept-Ranges":       "bytes",
				"Content-Disposition": `attachment; filename=export.xml`,
			},
		},
		{
			name:        "headers without filename",
			contentType: "image/png",
			fileSize:    2048,
			filename:    "",
			expectHeaders: map[string]string{
				"Content-Type":   "image/png",
				"Content-Length": "2048",
				"Cache-Control":  "no-cache, no-store, must-revalidate",
				"Pragma":         "no-cache",
				"Expires":        "0",
				"Accept-Ranges":  "bytes",
			},
		},
		{
			name:        "zero size file",
			contentType: "text/plain",
			fileSize:    0,
			filename:    "empty.txt",
			expectHeaders: map[string]string{
				"Content-Type":        "text/plain",
				"Content-Length":      "0",
				"Cache-Control":       "no-cache, no-store, must-revalidate",
				"Pragma":              "no-cache",
				"Expires":             "0",
				"Accept-Ranges":       "bytes",
				"Content-Disposition": `attachment; filename=empty.txt`,
			},
		},
		{
			name:        "large file",
			contentType: "application/pdf",
			fileSize:    1024 * 1024 * 10, // 10MB
			filename:    "manual.pdf",
			expectHeaders: map[string]string{
				"Content-Type":        "application/pdf",
				"Content-Length":      strconv.FormatInt(1024*1024*10, 10),
				"Cache-Control":       "no-cache, no-store, must-revalidate",
				"Pragma":              "no-cache",
				"Expires":             "0",
				"Accept-Ranges":       "bytes",
				"Content-Disposition": `attachment; filename=manual.pdf`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			rr := httptest.NewRecorder()

			downloadutils.SetStreamingHeaders(rr, tt.contentType, tt.fileSize, tt.filename)

			// Check all expected headers
			for key, expectedValue := range tt.expectHeaders {
				actualValue := rr.Header().Get(key)
				c.Assert(actualValue, qt.Equals, expectedValue, qt.Commentf("Header %s", key))
			}

			// Ensure Content-Disposition is not set when filename is empty
			if tt.filename == "" {
				c.Assert(rr.Header().Get("Content-Disposition"), qt.Equals, "")
			}
		})
	}
}
