//go:build !with_frontend

package apiserver

import "net/http"

// FrontendHandler returns a 404 handler when the binary is built without an
// embedded frontend. There is nothing to serve.
func FrontendHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
}
