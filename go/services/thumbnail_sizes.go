package services

import (
	"cmp"
	"maps"
	"slices"
)

// thumbnailSizes is the one list of thumbnail sizes: each name with the
// longest side of the rendered JPEG, in pixels. Every JPEG and PNG upload is
// rendered at each size, every signed file URL carries one thumbnail URL per
// size, and the name is the last segment of the thumbnail URL
// (/files/download/thumbnails/{fileID}/{size}) and of the thumbnail blob key.
//
// `large` serves the photo cards of the items list and the cover on the item
// page, which are wider than `medium` on a high-density screen.
var thumbnailSizes = map[string]int{
	"small":  150,
	"medium": 300,
	"large":  600,
}

// ThumbnailSizeNames returns the thumbnail size names, smallest first.
func ThumbnailSizeNames() []string {
	return slices.SortedFunc(maps.Keys(thumbnailSizes), func(a, b string) int {
		return cmp.Compare(thumbnailSizes[a], thumbnailSizes[b])
	})
}

// IsThumbnailSize reports whether name is one of the thumbnail sizes.
func IsThumbnailSize(name string) bool {
	_, ok := thumbnailSizes[name]
	return ok
}
