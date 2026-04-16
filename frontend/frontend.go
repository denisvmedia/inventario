//go:build with_frontend

package frontend

import (
	"embed"
	"io/fs"
)

// The "all:" prefix is required so that Go's embed includes files whose names
// begin with "_" or ".". Without it those files are silently skipped, which
// breaks the SPA: Vite's code-splitter emits chunks with underscore-prefixed
// names (e.g. _plugin-vue_export-helper-*.js), and missing chunks cause
// FrontendHandler to fall back to serving index.html with Content-Type
// text/html instead of the actual JavaScript, resulting in browser errors.
//
//go:embed all:dist
var dist embed.FS

func GetDist() fs.ReadFileFS {
	return dist
}
