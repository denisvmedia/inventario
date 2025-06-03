//go:build with_frontend

package frontend

import (
	"embed"
	"io/fs"
)

//go:embed dist
var dist embed.FS

func GetDist() fs.ReadFileFS {
	return dist
}
