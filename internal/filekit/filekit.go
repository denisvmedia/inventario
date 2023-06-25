package filekit

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

var NowFunc = time.Now

func UploadFileName(fileName string) string {
	fileExt := getMultiPartExtension(fileName)
	originalFileName := strings.TrimSuffix(
		filepath.Base(fileName),
		fileExt,
	)
	if originalFileName == "" {
		originalFileName = "h"
	}
	now := NowFunc()
	cleanFileName := strings.ReplaceAll(
		strings.ToLower(originalFileName),
		" ",
		"-",
	) + "-" + fmt.Sprintf("%v", now.Unix()) + fileExt
	return cleanFileName
}

func getMultiPartExtension(filePath string) string {
	ext := filepath.Ext(filePath)                 // Get the last element of the path
	filename := strings.TrimSuffix(filePath, ext) // Remove the extension from the filename
	multiPartExt := filepath.Ext(filename) + ext  // Combine the extension with the remaining filename
	return multiPartExt
}
