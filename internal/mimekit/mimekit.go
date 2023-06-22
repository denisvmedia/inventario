package mimekit

import (
	"github.com/gabriel-vasile/mimetype"
)

var imageContentTypes = []string{
	"image/gif",
	"image/jpeg",
	"image/png",
	"image/webp",
}

var docContentTypes = []string{
	"application/pdf",
}

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

func ExtensionByMime(mimeType string) string {
	return mimetype.Lookup(mimeType).Extension()
}
