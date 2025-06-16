package mimekit

import (
	"mime"

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
	for _, v := range imageContentTypes {
		if v == contentType {
			return true
		}
	}
	return false
}

func IsDoc(contentType string) bool {
	for _, v := range docContentTypes {
		if v == contentType {
			return true
		}
	}
	return false
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
