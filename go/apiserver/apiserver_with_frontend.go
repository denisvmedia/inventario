//go:build with_frontend

package apiserver

import (
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/denisvmedia/inventario/frontend"
	frontendreact "github.com/denisvmedia/inventario/frontend-react"
)

// Frontend bundle identifiers used by FrontendHandler. The validated form is
// owned by cmd/inventario/run/bootstrap; this package keeps a private mirror
// to avoid depending on cmd/* from apiserver/*. Keep these constants in sync
// with the ones in bootstrap/config.go.
const (
	frontendBundleLegacy = "legacy"
	frontendBundleNew    = "new"
)

// FrontendHandler returns the SPA handler for the requested bundle.
//
//   - "legacy": serves the Vue bundle from frontend/dist (today's behavior).
//   - "new":    serves the React bundle from frontend-react/dist.
//
// An unknown value is logged and falls back to "legacy" — bootstrap.ValidateFrontendBundle
// is the validation gate; this defense-in-depth keeps the binary serving
// something rather than a 500 if the gate is ever bypassed.
func FrontendHandler(bundle string) http.Handler {
	dist, root := selectBundle(bundle)
	fsys, _ := fs.Sub(dist, root)
	fileServer := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set correct MIME type for .mjs files (ES modules)
		if strings.HasSuffix(r.URL.Path, ".mjs") {
			w.Header().Set("Content-Type", "application/javascript")
		}

		recorder := httptest.NewRecorder()
		fileServer.ServeHTTP(recorder, r)
		if recorder.Code == http.StatusOK {
			// Copy headers from recorder to actual response
			for k, v := range recorder.Header() {
				// Don't overwrite Content-Type if we already set it for .mjs files
				if k == "Content-Type" && strings.HasSuffix(r.URL.Path, ".mjs") {
					continue
				}
				w.Header()[k] = v
			}
			w.WriteHeader(recorder.Code)
			_, _ = w.Write(recorder.Body.Bytes())
			return
		}
		data, err := dist.ReadFile(root + "/index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})
}

// selectBundle returns the embed FS and the dist directory name inside it for
// the requested bundle. Both bundles use a "dist" root because that's what
// each frontend's vite build produces; selectBundle is forward-compatible if
// that ever diverges.
func selectBundle(bundle string) (fs.ReadFileFS, string) {
	switch bundle {
	case frontendBundleNew:
		return frontendreact.GetDist(), "dist"
	case frontendBundleLegacy:
		return frontend.GetDist(), "dist"
	default:
		slog.Warn("Unknown frontend bundle requested; falling back to legacy",
			"bundle", bundle, "valid", []string{frontendBundleLegacy, frontendBundleNew})
		return frontend.GetDist(), "dist"
	}
}
