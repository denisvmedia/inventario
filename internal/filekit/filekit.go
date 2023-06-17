package filekit

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

var NowFunc = time.Now

func UploadFileName(fileName string) string {
	fileExt := filepath.Ext(fileName)
	originalFileName := strings.TrimSuffix(
		filepath.Base(fileName),
		fileExt,
	)
	now := NowFunc()
	cleanFileName := strings.ReplaceAll(
		strings.ToLower(originalFileName),
		" ",
		"-",
	) + "-" + fmt.Sprintf("%v", now.Unix()) + fileExt
	return cleanFileName
}
