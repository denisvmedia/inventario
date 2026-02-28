package apiserver

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"
)

func buildPublicURL(publicBaseURL, path string, query url.Values) (string, error) {
	base := strings.TrimSpace(publicBaseURL)
	if base == "" {
		return "", fmt.Errorf("public URL is required")
	}

	parsed, err := url.Parse(base)
	switch {
	case err != nil:
		slog.Error("Invalid public URL configuration; cannot build transactional email link",
			"public_url", base,
			"error", err,
		)
		return "", fmt.Errorf("parse public URL: %w", err)
	case parsed.Scheme == "" || parsed.Host == "":
		err := fmt.Errorf("scheme and host are required")
		slog.Error("Invalid public URL configuration; cannot build transactional email link",
			"public_url", base,
			"error", err,
		)
		return "", err
	}

	scheme := strings.ToLower(parsed.Scheme)
	if !isAllowedPublicURLScheme(scheme) {
		err := fmt.Errorf("unsupported scheme %q", parsed.Scheme)
		slog.Error("Invalid public URL configuration; only http/https schemes are allowed",
			"public_url", base,
			"error", err,
		)
		return "", err
	}

	parsed.Scheme = scheme
	return parsed.ResolveReference(&url.URL{
		Path:     path,
		RawQuery: query.Encode(),
	}).String(), nil
}

func isAllowedPublicURLScheme(scheme string) bool {
	return scheme == "http" || scheme == "https"
}
