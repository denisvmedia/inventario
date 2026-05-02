//go:build with_frontend

package apiserver

import (
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/denisvmedia/inventario/frontend"
)

// FrontendHandler returns the SPA handler that serves the embedded React
// bundle from frontend/dist.
func FrontendHandler() http.Handler {
	dist := frontend.GetDist()
	fsys, err := fs.Sub(dist, "dist")
	if err != nil {
		// Embed layout drift (root directory renamed/missing) is the only
		// way fs.Sub can error in practice. Return a 500 handler instead of
		// proceeding with a nil FS that http.FileServer would panic on.
		slog.Error("Failed to prepare frontend filesystem", "err", err)
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
		data, err := fs.ReadFile(dist, "dist/index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})
}
