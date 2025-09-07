//go:build with_frontend

package apiserver

import (
	"io/fs"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/denisvmedia/inventario/frontend"
)

func init() {
	// Register .mjs MIME type for ES modules
	mime.AddExtensionType(".mjs", "application/javascript")
}

func FrontendHandler() http.Handler {
	dist := frontend.GetDist()
	fsys, _ := fs.Sub(dist, "dist")
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
		data, err := dist.ReadFile("dist/index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})
}
