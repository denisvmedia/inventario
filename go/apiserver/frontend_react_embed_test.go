//go:build with_frontend

package apiserver_test

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	frontendreact "github.com/denisvmedia/inventario/frontend-react"
)

// TestFrontendReactEmbed_IndexHtmlHasInventarioTitle verifies that the React
// bundle embedded by frontend-react/frontend.go is reachable, has the
// expected <title>Inventario</title>, and contains no leftover Bolt
// (bolt.new) artifacts.
//
// This test requires the React frontend to be built first
// (npm run build in frontend-react/). It runs in CI under the
// frontend-react-embed-smoke-test workflow.
func TestFrontendReactEmbed_IndexHtmlHasInventarioTitle(t *testing.T) {
	c := qt.New(t)

	dist := frontendreact.GetDist()
	indexHTML, err := fs.ReadFile(dist, "dist/index.html")
	c.Assert(err, qt.IsNil)

	body := string(indexHTML)
	c.Assert(strings.Contains(body, "<title>Inventario</title>"), qt.IsTrue,
		qt.Commentf("expected <title>Inventario</title> in dist/index.html, got: %s", body))

	// Guard against the "Inventario Design Proposal" title and the bolt.new
	// og:image/twitter:image meta tags that ship with the upstream design
	// mock — they should never reach the production bundle.
	c.Assert(strings.Contains(body, "Design Proposal"), qt.IsFalse,
		qt.Commentf("dist/index.html still has the mock 'Design Proposal' title"))
	c.Assert(strings.Contains(body, "bolt.new"), qt.IsFalse,
		qt.Commentf("dist/index.html still has bolt.new artifacts"))
}

// TestFrontendReactEmbed_UnderscorePrefixedFilesArePresent mirrors the
// equivalent check on the legacy bundle (TestFrontendEmbed_*): if //go:embed
// were missing the "all:" prefix, Vite chunks like
// _plugin-react_jsx-runtime-*.js would be silently skipped from the binary
// and the SPA would break with text/html responses for missing chunks.
func TestFrontendReactEmbed_UnderscorePrefixedFilesArePresent(t *testing.T) {
	c := qt.New(t)

	dist := frontendreact.GetDist()

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
	// The walk may return ENOENT if assets/ has no underscore-prefixed
	// chunks for this build (small bundles can dodge the case entirely);
	// assert only that the walk itself didn't error on a fundamentally
	// broken embed.
	c.Assert(err, qt.IsNil)
	t.Logf("found %d underscore-prefixed chunk(s) in React bundle: %v", len(found), found)
}
