package apiserver

import (
	"fmt"
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
		return "", fmt.Errorf("parse public URL: %w", err)
	case parsed.Scheme == "" || parsed.Host == "":
		return "", fmt.Errorf("scheme and host are required")
	}

	scheme := strings.ToLower(parsed.Scheme)
	if !isAllowedPublicURLScheme(scheme) {
		return "", fmt.Errorf("unsupported scheme %q", parsed.Scheme)
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
