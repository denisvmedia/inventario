package apiserver

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

func buildPublicURL(publicBaseURL string, r *http.Request, path string, query url.Values) string {
	base := strings.TrimSpace(publicBaseURL)
	fallbackReason := "public_url_not_set"
	if base != "" {
		parsed, err := url.Parse(base)
		switch {
		case err != nil:
			fallbackReason = "public_url_parse_error"
			slog.Error("Invalid public URL configuration; falling back to request host",
				"public_url", base,
				"error", err,
			)
		case parsed.Scheme != "" && parsed.Host != "":
			scheme := strings.ToLower(parsed.Scheme)
			if !isAllowedPublicURLScheme(scheme) {
				fallbackReason = "public_url_unsupported_scheme"
				slog.Error("Invalid public URL configuration; only http/https schemes are allowed",
					"public_url", base,
					"scheme", parsed.Scheme,
				)
				break
			}
			parsed.Scheme = scheme
			return parsed.ResolveReference(&url.URL{
				Path:     path,
				RawQuery: query.Encode(),
			}).String()
		default:
			fallbackReason = "public_url_missing_scheme_or_host"
			slog.Error("Invalid public URL configuration; scheme and host are required",
				"public_url", base,
			)
		}
	}

	slog.Warn("Using request host to build public URL; configure --public-url to avoid host-header-derived links",
		"reason", fallbackReason,
		"host", r.Host,
	)

	scheme := "http"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		first := strings.TrimSpace(strings.Split(proto, ",")[0])
		first = strings.ToLower(first)
		if isAllowedPublicURLScheme(first) {
			scheme = first
		} else {
			slog.Warn("Ignoring unsupported X-Forwarded-Proto value", "value", proto)
		}
	} else if r.TLS != nil {
		scheme = "https"
	}
	return (&url.URL{
		Scheme:   scheme,
		Host:     r.Host,
		Path:     path,
		RawQuery: query.Encode(),
	}).String()
}

func isAllowedPublicURLScheme(scheme string) bool {
	return scheme == "http" || scheme == "https"
}
