package textutils_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/textutils"
)

func TestCleanFilename(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected string
		}{
			{
				name:     "normal filename",
				input:    "normal_filename.txt",
				expected: "normal_filename_txt",
			},
			{
				name:     "filename with spaces",
				input:    "file name with spaces.txt",
				expected: "file-name-with-spaces_txt",
			},
			{
				name:     "trim spaces",
				input:    "  filename_with_spaces_around  ",
				expected: "filename_with_spaces_around",
			},
			{
				name:     "replace dots and slashes",
				input:    "file.with.dots/and/slashes\\",
				expected: "file_with_dotsandslashes",
			},
			{
				name:     "with emoji",
				input:    "file with 🚀 emoji",
				expected: "file-with-rocket-emoji",
			},
			{
				name:     "with emoji at start",
				input:    "🚀 rocket at start",
				expected: "rocket-rocket-at-start",
			},
			{
				name:     "with emoji at end",
				input:    "rocket at end 🚀",
				expected: "rocket-at-end-rocket",
			},
			{
				name:     "with BOM",
				input:    "\uFEFFfilename_with_bom",
				expected: "filename_with_bom",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				c := qt.New(t)
				result := textutils.CleanFilename(test.input)
				c.Assert(result, qt.Equals, test.expected)
			})
		}
	})

	t.Run("unhappy path", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected string
		}{
			{
				name:     "with unsafe characters",
				input:    "file<with>unsafe:characters?",
				expected: "filewithunsafecharacters",
			},
			{
				name:     "with control characters",
				input:    "file\u0000with\u0001control\u0002chars",
				expected: "filewithcontrolchars",
			},
			{
				name:     "with non-graphic characters",
				input:    "file\u0080with\u0081non\u0082graphic\u0083chars",
				expected: "filewithnongraphicchars",
			},
			{
				name:     "empty string",
				input:    "",
				expected: "",
			},
			{
				name:     "only unsafe characters",
				input:    "<>:\\/|?*",
				expected: "",
			},
			{
				name:     "with utf8 rune error",
				input:    string([]byte{0xFF, 0xFE, 0xFD}) + "valid",
				expected: "valid",
			},
			{
				name:     "path traversal attempt",
				input:    "../../../etc/passwd",
				expected: "_etcpasswd",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				c := qt.New(t)
				result := textutils.CleanFilename(test.input)
				c.Assert(result, qt.Equals, test.expected)
			})
		}
	})
}

func TestGetEmojiName(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		tests := []struct {
			name     string
			emoji    rune
			expected string
		}{
			{
				name:     "rocket emoji",
				emoji:    '🚀',
				expected: "rocket",
			},
			{
				name:     "smiling face emoji",
				emoji:    '😀',
				expected: "grinning",
			},
			{
				name:     "thumbs up emoji",
				emoji:    '👍',
				expected: "_1",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				c := qt.New(t)
				result := textutils.GetEmojiName(test.emoji)
				c.Assert(result, qt.Equals, test.expected)
			})
		}
	})

	t.Run("unhappy path", func(t *testing.T) {
		tests := []struct {
			name  string
			input rune
		}{
			{
				name:  "regular letter",
				input: 'A',
			},
			{
				name:  "number",
				input: '1',
			},
			{
				name:  "special character",
				input: '@',
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				c := qt.New(t)
				result := textutils.GetEmojiName(test.input)
				c.Assert(result, qt.Equals, "")
			})
		}
	})
}
