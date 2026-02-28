package apiserver

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

func buildPublicURL(publicBaseURL string, r *http.Request, path string, query url.Values) string {
	base := strings.TrimSpace(publicBaseURL)
	if base != "" {
		parsed, err := url.Parse(base)
		switch {
		case err != nil:
			slog.Error("Invalid public URL configuration; falling back to request host",
				"public_url", base,
				"error", err,
			)
		case parsed.Scheme != "" && parsed.Host != "":
			return parsed.ResolveReference(&url.URL{
				Path:     path,
				RawQuery: query.Encode(),
			}).String()
		default:
			slog.Error("Invalid public URL configuration; scheme and host are required",
				"public_url", base,
			)
		}
	}

	scheme := "http"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
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
