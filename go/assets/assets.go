package assets

import (
	"embed"
	"io/fs"
)

//go:embed placeholders
var placeholders embed.FS

// GetPlaceholders returns the embedded placeholders filesystem
func GetPlaceholders() fs.FS {
	return placeholders
}

// GetPlaceholderFile reads a placeholder file by name
func GetPlaceholderFile(filename string) ([]byte, error) {
	return placeholders.ReadFile("placeholders/" + filename)
}
