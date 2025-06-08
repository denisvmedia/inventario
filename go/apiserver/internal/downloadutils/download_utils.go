// Package downloadutils provides utilities for optimized file downloads with streaming support.
package downloadutils

import (
	"context"
	"io"
	"net/http"
	"strconv"

	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/mimekit"
)

// GetFileAttributes retrieves file attributes including size for setting Content-Length header.
// This function opens a bucket connection, retrieves the file attributes, and closes the connection.
func GetFileAttributes(ctx context.Context, uploadLocation, filePath string) (*blob.Attributes, error) {
	b, err := blob.OpenBucket(ctx, uploadLocation)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to open bucket")
	}
	defer b.Close()

	attrs, err := b.Attributes(ctx, filePath)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get file attributes")
	}

	return attrs, nil
}

// CopyFileInChunks copies file data in chunks to prevent browser buffering and improve streaming.
// It reads data in 32KB chunks and flushes the response writer after each chunk when possible.
// This approach prevents accumulation of large amounts of data in memory and provides better
// streaming performance for large files.
func CopyFileInChunks(w http.ResponseWriter, r io.Reader) error {
	// Use 32KB chunks for optimal streaming performance
	const chunkSize = 32 * 1024
	buf := make([]byte, chunkSize)

	for {
		n, err := r.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return errkit.Wrap(writeErr, "failed to write chunk to response")
			}
			// Flush the response writer if it supports flushing
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return errkit.Wrap(err, "failed to read chunk from file")
		}
	}
	return nil
}

// SetStreamingHeaders sets HTTP headers optimized for streaming downloads and preventing browser preloading.
// It sets the following headers:
// - Content-Type: specified content type
// - Content-Length: file size for proper download progress indication
// - Cache-Control: prevents browser caching of large files
// - Pragma: legacy cache control for older browsers
// - Expires: ensures immediate expiration
// - Accept-Ranges: indicates support for range requests
// - Content-Disposition: sets attachment filename when provided
func SetStreamingHeaders(w http.ResponseWriter, contentType string, fileSize int64, filename string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Accept-Ranges", "bytes")

	if filename != "" {
		attachmentHeader := mimekit.FormatContentDisposition(filename)
		w.Header().Set("Content-Disposition", attachmentHeader)
	}
}
