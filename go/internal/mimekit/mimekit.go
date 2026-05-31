package mimekit

import (
	"mime"
	"slices"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

var imageContentTypes = []string{
	"image/gif",
	"image/jpeg",
	"image/png",
	"image/webp",
}

var xmlContentTypes = []string{
	"application/xml",
	"text/xml",
}

// inlineSafeContentTypes is the allowlist of content types we are willing
// to serve with `Content-Disposition: inline` on the same-origin signed
// file route (the "Open in new tab" affordance, #1962). It is deliberately
// narrow: types the browser renders for viewing and that cannot execute
// script in our origin. text/html and image/svg+xml are EXCLUDED on
// purpose — they can carry active content and would be a stored-XSS vector
// if rendered inline same-origin. Anything outside this set falls back to
// an attachment download even when an inline serve was requested.
var inlineSafeContentTypes = append(append(
	[]string(nil),
	imageContentTypes...,
), "application/pdf", "text/plain")

// INBMIMEType is the canonical media type for a signed `.inb` backup
// archive (issue #534). The archive is an uncompressed outer tar, so a
// content sniffer rarely identifies it as anything more specific than
// application/octet-stream — hence INBContentTypes also accepts that.
const INBMIMEType = "application/x-inventario-backup"

var inbContentTypes = []string{
	INBMIMEType,
	// `.inb` is an uncompressed outer tar, which content sniffers identify as
	// application/x-tar. Accept that as the primary detected type.
	"application/x-tar",
	// Some sniffers/edge cases fall back to a generic binary stream, so accept
	// octet-stream too. The restore path still hard-verifies the signature, so a
	// mislabelled upload cannot bypass any security check.
	"application/octet-stream",
}

var docContentTypes = append(append(
	[]string(nil),
	imageContentTypes...,
), "application/pdf")

var allContentTypes = append(append(append(
	[]string(nil),
	imageContentTypes...,
), "application/pdf"),
	// Add more common content types for files
	"text/plain",
	"text/csv",
	"application/json",
	"application/zip",
	"application/x-zip-compressed",
	"application/vnd.ms-excel",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"application/msword",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"application/vnd.ms-powerpoint",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation",
	"video/mp4",
	"video/avi",
	"video/quicktime",
	"video/x-msvideo",
	"audio/mpeg",
	"audio/wav",
	"audio/x-wav",
	"audio/mp3",
)

func IsImage(contentType string) bool {
	return slices.Contains(imageContentTypes, contentType)
}

// IsInlineSafe reports whether contentType may be served with
// Content-Disposition: inline for in-browser viewing. Everything outside
// the narrow allowlist (see inlineSafeContentTypes) must fall back to an
// attachment download even when an inline serve was requested, so that
// active content (HTML, SVG) can never execute in our origin.
//
// The content type is normalised before the lookup — parameters are
// stripped and the media type is lowercased — so a stored value like
// "text/plain; charset=utf-8" or "IMAGE/PNG" still matches the allowlist
// (and, conversely, "text/html; charset=utf-8" still does not). An empty
// or unparseable value falls through to the download case.
func IsInlineSafe(contentType string) bool {
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = mediaType
	}
	return slices.Contains(inlineSafeContentTypes, strings.ToLower(contentType))
}

func IsDoc(contentType string) bool {
	return slices.Contains(docContentTypes, contentType)
}

func ImageContentTypes() []string {
	result := make([]string, len(imageContentTypes))
	copy(result, imageContentTypes)
	return result
}

func DocContentTypes() []string {
	result := make([]string, len(docContentTypes))
	copy(result, docContentTypes)
	return result
}

func XMLContentTypes() []string {
	result := make([]string, len(xmlContentTypes))
	copy(result, xmlContentTypes)
	return result
}

// INBContentTypes returns the content types accepted for a `.inb` backup
// upload (issue #534): the custom INBMIMEType plus application/octet-stream
// for the common case where the sniffer can't identify the bare tar.
func INBContentTypes() []string {
	result := make([]string, len(inbContentTypes))
	copy(result, inbContentTypes)
	return result
}

func AllContentTypes() []string {
	result := make([]string, len(allContentTypes))
	copy(result, allContentTypes)
	return result
}

func ExtensionByMime(mimeType string) string {
	m := mimetype.Lookup(mimeType)
	if m == nil {
		return ".unknown"
	}

	return m.Extension()
}

func FormatContentDisposition(filename string) string {
	params := map[string]string{
		"filename": filename,
	}
	return mime.FormatMediaType("attachment", params)
}

// FormatInlineContentDisposition formats a Content-Disposition header that
// asks the browser to render the file inline (for viewing in a new tab,
// #1962) rather than download it. Callers MUST gate this on IsInlineSafe —
// it is never correct to emit inline for active content (HTML / SVG).
func FormatInlineContentDisposition(filename string) string {
	params := map[string]string{
		"filename": filename,
	}
	return mime.FormatMediaType("inline", params)
}
