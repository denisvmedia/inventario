//go:build with_frontend

package apiserver_test

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/frontend"
)

// TestFrontendEmbed_UnderscorePrefixedFilesArePresent verifies that files
// with underscore-prefixed names are included in the embedded filesystem
// returned by frontend.GetDist().
//
// Go's //go:embed silently skips any file whose name begins with "_" or "."
// unless the directive uses the "all:" prefix. Without it those files are
// absent from the binary, FrontendHandler falls back to serving index.html
// for those paths, and the browser receives Content-Type: text/html instead
// of the actual asset — breaking the SPA entirely.
//
// We rely on a stable marker file shipped at frontend/public/_inventario-embed.txt
// (Vite copies the public/ tree verbatim into dist/) so the assertion is
// independent of the bundler and its chunk-naming scheme. Any underscore-
// prefixed Vite chunk that happens to be present is also accepted.
//
// This test requires the frontend to be built first (npm run build in frontend/).
// It is executed in CI by the frontend-embed-smoke-test workflow.
func TestFrontendEmbed_UnderscorePrefixedFilesArePresent(t *testing.T) {
	c := qt.New(t)

	dist := frontend.GetDist()

	var found []string
	err := fs.WalkDir(dist, "dist", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasPrefix(filepath.Base(path), "_") {
			found = append(found, path)
		}
		return nil
	})
	c.Assert(err, qt.IsNil)
	c.Assert(len(found) > 0, qt.IsTrue,
		qt.Commentf("no underscore-prefixed files found in embedded dist — "+
			"ensure the //go:embed directive in frontend/frontend.go uses 'all:dist' "+
			"and that frontend/public/_inventario-embed.txt is still present"))

	// The marker file must be present specifically — it's the stable proof
	// the all: prefix is doing its job, regardless of bundler.
	c.Assert(found, qt.Contains, "dist/_inventario-embed.txt",
		qt.Commentf("marker file missing from embedded dist — see "+
			"frontend/public/_inventario-embed.txt"))

	t.Logf("found %d underscore-prefixed file(s): %v", len(found), found)
}
