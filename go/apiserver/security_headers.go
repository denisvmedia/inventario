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
// The Content-Security-Policy below is a constant and not a knob, because
// nothing in the product loads from another origin. Signed file URLs are
// same-origin (`/api/v1/files/download/…` — the server streams the bytes
// rather than redirecting to the object store), there is no presigned
// direct-to-bucket upload, and the frontend makes no cross-origin fetches.
// A deployment that needs to relax it can have a flag when it exists; a knob
// added ahead of that is one more way to ship a policy nobody checked.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	// Radix, sonner and friends append <style> elements at runtime. React's
	// `style` prop goes through CSSOM and is unaffected either way; it is the
	// injected elements that need this. script-src stays strict, which is
	// where XSS actually lives.
	"style-src 'self' 'unsafe-inline'; " +
	// data: and blob: are the locally-previewed upload, before it is sent.
	"img-src 'self' data: blob:; " +
	"font-src 'self' data:; " +
	"connect-src 'self'; " +
	// pdf.js runs its parser in a worker. The bundler emits it as a
	// same-origin asset, which default-src already covers; blob: is there
	// because pdf.js falls back to a blob worker in some paths and the
	// failure would be a PDF that silently never renders.
	"worker-src 'self' blob:; " +
	"object-src 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'; " +
	// Supersedes X-Frame-Options for browsers that honor it; the older header
	// stays for the proxies and scanners that read only that one.
	"frame-ancestors 'none'"

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
			h.Set("Content-Security-Policy", contentSecurityPolicy)

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
