package apiserver

import (
	"net"
	"net/http"
	"strings"
)

// TailnetHostRedirect issues a 301 redirect when the request arrives on a
// short hostname that should resolve to a fully-qualified MagicDNS name —
// e.g. `http://inv-vcl01-pr1881` → `https://inv-vcl01-pr1881.<TAILNET>.ts.net`.
//
// Rationale: Tailscale-issued TLS certificates only cover the device's full
// FQDN (`<short>.<tailnet>.ts.net`). The Tailscale Operator's built-in
// HTTP→HTTPS redirect (`tailscale.com/http-redirect: "true"`) preserves the
// requested Host, so a browser hitting `http://<short>/` gets bounced to
// `https://<short>/` and fails TLS validation. By landing this middleware
// in front of the chi router, we rewrite the redirect target to include the
// tailnet suffix so the browser ends up at a working URL with a valid cert.
//
// suffix: the tailnet MagicDNS suffix, e.g. "<TAILNET>.ts.net". When
// empty, the middleware no-ops — the Tailscale Operator's same-host redirect
// stays the only handler, which is the right behavior for non-tailnet
// deployments (cert-manager + a public domain → request and cert share a
// host).
//
// This is intentionally checked BEFORE the route matches: the middleware
// short-circuits on host alone, so unauthenticated browsers landing on the
// short URL get redirected without ever exercising the rest of the stack.
func TailnetHostRedirect(suffix string) func(http.Handler) http.Handler {
	suffix = strings.TrimSpace(suffix)
	dotSuffix := ""
	if suffix != "" {
		dotSuffix = "." + strings.TrimPrefix(suffix, ".")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if dotSuffix == "" {
				next.ServeHTTP(w, r)
				return
			}
			host, _, err := net.SplitHostPort(r.Host)
			if err != nil {
				// r.Host has no port — treat the whole value as the host.
				host = r.Host
			}
			if host == "" {
				next.ServeHTTP(w, r)
				return
			}
			// Skip IPs (literal v4 or v6) — they never resolve to an FQDN
			// in MagicDNS and a redirect would only confuse the client.
			if net.ParseIP(host) != nil {
				next.ServeHTTP(w, r)
				return
			}
			// Already a fully-qualified MagicDNS name (or any other suffix
			// match) — let the default handler do whatever it does.
			lowerHost := strings.ToLower(host)
			lowerSuffix := strings.ToLower(dotSuffix)
			if strings.HasSuffix(lowerHost, lowerSuffix) {
				next.ServeHTTP(w, r)
				return
			}
			// Rewrite: redirect to https://<host>.<suffix><path>?<query>
			target := "https://" + host + dotSuffix + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		})
	}
}
