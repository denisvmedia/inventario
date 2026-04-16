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

// TestFrontendEmbed_UnderscorePrefixedFilesArePresent verifies that Vite chunks
// with underscore-prefixed names (e.g. _plugin-vue_export-helper-*.js) are
// included in the embedded filesystem returned by frontend.GetDist().
//
// Go's //go:embed silently skips any file whose name begins with "_" or "."
// unless the directive uses the "all:" prefix. Without it the chunks are absent
// from the binary, FrontendHandler falls back to serving index.html for those
// paths, and the browser receives Content-Type: text/html instead of
// application/javascript — breaking the SPA entirely.
//
// This test requires the frontend to be built first (npm run build in frontend/).
// It is executed in CI by the frontend-embed-smoke-test workflow.
func TestFrontendEmbed_UnderscorePrefixedFilesArePresent(t *testing.T) {
	c := qt.New(t)

	dist := frontend.GetDist()

	var found []string
	err := fs.WalkDir(dist, "dist/assets", func(path string, d fs.DirEntry, err error) error {
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
		qt.Commentf("no underscore-prefixed Vite chunks found in embedded dist/assets — "+
			"ensure the //go:embed directive in frontend/frontend.go uses 'all:dist'"))

	t.Logf("found %d underscore-prefixed chunk(s): %v", len(found), found)
}
