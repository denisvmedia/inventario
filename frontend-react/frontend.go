//go:build with_frontend

// Package frontendreact embeds the React frontend bundle (dist/) into the Go
// binary so a single binary can serve either the legacy Vue bundle or the new
// React bundle based on the INVENTARIO_FRONTEND env var (wired in #1401).
//
// The "all:" prefix on //go:embed is required so files whose names begin with
// "_" or "." are included; without it Vite's underscore-prefixed chunks
// (e.g. _plugin-react_jsx-runtime-*.js) would be silently skipped, breaking
// the SPA in the same way it would for the legacy bundle.
package frontendreact

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
