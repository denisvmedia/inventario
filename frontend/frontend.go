//go:build with_frontend

// Package frontend embeds the React frontend bundle (dist/) into the Go
// binary so the binary serves the SPA at the root path.
//
// The "all:" prefix on //go:embed is required so files whose names begin with
// "_" or "." are included; without it Vite's underscore-prefixed chunks
// (e.g. _plugin-react_jsx-runtime-*.js) would be silently skipped, breaking
// the SPA.
package frontend

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// GetDist returns the embedded dist/ filesystem produced by `npm run build`.
func GetDist() fs.ReadFileFS {
	return dist
}
