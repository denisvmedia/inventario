//go:build !with_frontend

package apiserver

import "net/http"

// FrontendHandler returns a 404 handler when the binary is built without an
// embedded frontend. The bundle argument is accepted for signature parity
// with the with_frontend build but is ignored — there is nothing to serve.
func FrontendHandler(_ string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
}
