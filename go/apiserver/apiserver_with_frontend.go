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
	"github.com/denisvmedia/inventario/internal/frontendbundle"
)

// FrontendHandler returns the SPA handler for the requested bundle.
//
//   - "legacy": serves the Vue bundle from frontend/dist (today's behavior).
//   - "new":    serves the React bundle from frontend-react/dist.
//
// An unknown value is logged and falls back to "legacy" — frontendbundle.Validate
// is the actual validation gate (run from bootstrap before any HTTP listener
// starts); this defense-in-depth keeps the binary serving something rather
// than a 500 if the gate is ever bypassed.
func FrontendHandler(bundle string) http.Handler {
	dist, root := selectBundle(bundle)
	fsys, err := fs.Sub(dist, root)
	if err != nil {
		// Embed layout drift (root directory renamed/missing) is the only
		// way fs.Sub can error in practice. Return a 500 handler instead of
		// proceeding with a nil FS that http.FileServer would panic on.
		slog.Error("Failed to prepare frontend filesystem",
			"bundle", bundle, "root", root, "err", err)
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		})
	}
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
		data, err := fs.ReadFile(dist, root+"/index.html")
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
func selectBundle(bundle string) (fs.FS, string) {
	switch bundle {
	case frontendbundle.New:
		return frontendreact.GetDist(), "dist"
	case frontendbundle.Legacy:
		return frontend.GetDist(), "dist"
	default:
		slog.Warn("Unknown frontend bundle requested; falling back to legacy",
			"bundle", bundle, "valid", frontendbundle.Valid)
		return frontend.GetDist(), "dist"
	}
}
