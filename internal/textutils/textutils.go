package textutils

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mrosales/emoji-go"
)

var emojiIndex = (func() map[string]string {
	result := make(map[string]string)
	for _, e := range emoji.All {
		result[e.Character] = e.Name
	}
	return result
})()

var safeRe = regexp.MustCompile(`[./\\]+`)

// GetEmojiName returns the name of an emoji rune if it is an emoji, otherwise returns an empty string.
func GetEmojiName(r rune) string {
	// Only do direct character matching
	em := emojiIndex[string(r)]
	if em != "" {
		return em
	}

	return ""
}

// CleanFilename cleans a filename by removing all invalid characters and replacing spaces with underscores.
// It also removes all sequences of dots/slashes and trims spaces from the beginning and end.
// It also removes all sequences of dots/slashes and trims spaces from the beginning and end.
// It also replaces all emoji with their short name wrapped in underscores.
func CleanFilename(name string) string {
	// Remove BOM
	if strings.HasPrefix(name, "\uFEFF") {
		name = strings.TrimPrefix(name, "\uFEFF")
	}

	// Iterate over all runes and exclude the unwanted ones
	var b strings.Builder
	runes := []rune(name)
	for _, r := range runes {
		if r == utf8.RuneError {
			continue
		}
		if !isRuneSafe(r) {
			continue
		}
		emojiName := GetEmojiName(r)
		if emojiName != "" {
			b.WriteString(emojiName)
			continue
		}
		b.WriteRune(r)
	}

	// Remove spaces from the beginning and end
	safe := strings.TrimSpace(b.String())

	// Additionally, remove sequences of dots/slashes
	safe = safeRe.ReplaceAllString(safe, "_")

	safe = strings.ReplaceAll(
		strings.ToLower(safe),
		" ",
		"-",
	)

	return safe
}

// isRuneSafe проверяет, допустим ли символ в имени файла
func isRuneSafe(r rune) bool {
	if r < 32 || r == 127 {
		return false // управляющие символы
	}
	switch r {
	case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
		return false // зарезервированные
	}
	if !unicode.IsGraphic(r) {
		return false
	}
	return true
}
