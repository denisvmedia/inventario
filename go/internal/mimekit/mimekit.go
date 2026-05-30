package mimekit

import (
	"mime"
	"slices"

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
