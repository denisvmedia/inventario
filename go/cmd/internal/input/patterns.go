package input

import "regexp"

// Package-level compiled regex patterns for validation
var (
	emailPattern = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	slugPattern  = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	// Password validation patterns (Go doesn't support lookahead)
	hasLowerPattern = regexp.MustCompile(`[a-z]`)
	hasUpperPattern = regexp.MustCompile(`[A-Z]`)
	hasDigitPattern = regexp.MustCompile(`\d`)
)

// IsValidEmail checks if the given string is a valid email address
func IsValidEmail(email string) bool {
	return emailPattern.MatchString(email)
}

// IsValidSlug checks if the given string is a valid slug
func IsValidSlug(slug string) bool {
	return slugPattern.MatchString(slug)
}

// IsValidPassword checks if the given string meets password requirements
func IsValidPassword(password string) bool {
	if len(password) < 8 {
		return false
	}
	return hasLowerPattern.MatchString(password) &&
		hasUpperPattern.MatchString(password) &&
		hasDigitPattern.MatchString(password)
}
