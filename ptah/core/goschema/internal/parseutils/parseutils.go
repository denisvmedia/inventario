package parseutils

import (
	"regexp"
	"strings"
)

var keyValuePairRe = regexp.MustCompile(`(\w+(?:\.\w+)*)=(?:"([^"]*)"|([^\s]+))`)
var boolRe = regexp.MustCompile(`\b(\w+(?:\.\w+)*)\b`)

func ParseKeyValueComment(comment string) map[string]string {
	result := make(map[string]string)

	// First, handle key=value pairs (quoted and unquoted)
	matches := keyValuePairRe.FindAllStringSubmatch(comment, -1)
	for _, match := range matches {
		key := match[1]
		// match[2] is the quoted value (if quoted), match[3] is the unquoted value
		if match[2] != "" {
			result[key] = match[2] // Use quoted value
		} else {
			result[key] = match[3] // Use unquoted value
		}
	}

	// Then, handle standalone boolean attributes (no =value)
	// Remove all key=value pairs from the comment first
	cleanComment := keyValuePairRe.ReplaceAllString(comment, "")

	// Find standalone words that could be boolean flags
	boolMatches := boolRe.FindAllStringSubmatch(cleanComment, -1)

	// Known boolean attributes that can be standalone
	booleanAttrs := map[string]bool{
		"not_null": true, "nullable": true, "primary": true, "unique": true,
		"auto_increment": true, "index": true, "autoincrement": true,
	}

	for _, match := range boolMatches {
		attr := match[1]
		// Skip directive names and other non-boolean words
		if attr == "migrator" || attr == "schema" || attr == "field" ||
			attr == "table" || attr == "embed" || attr == "embedded" {
			continue
		}
		// Only treat as boolean if it's a known boolean attribute or follows boolean naming pattern
		if booleanAttrs[attr] || strings.HasSuffix(attr, "_null") ||
			strings.HasPrefix(attr, "is_") || strings.HasPrefix(attr, "has_") {
			// Only set if not already set by key=value parsing
			if _, exists := result[attr]; !exists {
				result[attr] = "true"
			}
		}
	}

	return result
}

func ParsePlatformSpecific(kv map[string]string) map[string]map[string]string {
	out := make(map[string]map[string]string)
	for k, v := range kv {
		// Only use platform. prefix, dropping override. completely
		if strings.HasPrefix(k, "platform.") {
			parts := strings.SplitN(k, ".", 3)

			if len(parts) == 3 {
				db := parts[1]
				key := parts[2]
				if _, ok := out[db]; !ok {
					out[db] = make(map[string]string)
				}
				out[db][key] = v
			}
		}

		// Move engine and comment to platform-specific attributes
		if k == "engine" {
			for _, dialect := range []string{"mysql", "mariadb"} {
				if _, ok := out[dialect]; !ok {
					out[dialect] = make(map[string]string)
				}
				out[dialect]["engine"] = v
			}
		}

		if k == "comment" {
			for _, dialect := range []string{"mysql", "mariadb"} {
				if _, ok := out[dialect]; !ok {
					out[dialect] = make(map[string]string)
				}
				out[dialect]["comment"] = v
			}
		}
	}
	return out
}
