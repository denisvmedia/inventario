package apiserver

import (
	"net/http"
	"strings"
)

// SecurityHeaders stamps the response headers a browser needs to be told
// explicitly, because their safe behaviour is not the default.
//
// It sits on the root router, so it covers the API and the embedded SPA
// alike. A reverse proxy in front may set the same headers; setting them here
// means a stock `docker compose up` — which serves the SPA straight from the
// binary with nothing in front of it — is not left without them.
//
// Content-Security-Policy is deliberately NOT here. A policy that is wrong
// breaks the application in the browser, in ways no test in this repository
// would catch, and two facts have to be settled first: uploads can be served
// from a separate origin (S3/R2 signed URLs), so `img-src 'self'` is wrong for
// some deployments; and several UI libraries inject `<style>` elements at
// runtime. It needs its own change, with the policy driven by the configured
// upload location.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			// Without it a browser may sniff a response body and execute an
			// uploaded file as script, whatever the declared Content-Type says.
			h.Set("X-Content-Type-Options", "nosniff")
			// Nothing in the product renders inside a frame, so clickjacking
			// has no legitimate case to preserve.
			h.Set("X-Frame-Options", "DENY")
			// The default leaks the full URL — including group slugs and entity
			// ids — to every cross-origin request the page makes.
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// HSTS only over TLS. Browsers ignore it on a plain-HTTP origin,
			// but sending it there would also pin `localhost` for developers
			// who run the stack over HTTP, which is a long-lived annoyance for
			// no benefit.
			if requestIsTLS(r) {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// requestIsTLS reports whether the request reached the server over HTTPS,
// directly or through a proxy that terminated TLS.
func requestIsTLS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
