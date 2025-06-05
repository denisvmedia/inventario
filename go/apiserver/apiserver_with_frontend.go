//go:build with_frontend

package apiserver

import (
	"io/fs"
	"net/http"
	"net/http/httptest"

	"github.com/denisvmedia/inventario/frontend"
)

func FrontendHandler() http.Handler {
	dist := frontend.GetDist()
	fsys, _ := fs.Sub(dist, "dist")
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := httptest.NewRecorder()
		fileServer.ServeHTTP(recorder, r)
		if recorder.Code == http.StatusOK {
			fileServer.ServeHTTP(w, r)
			return
		}
		data, err := dist.ReadFile("dist/index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	})
}
