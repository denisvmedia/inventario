package mimekit_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/mimekit"
)

func TestIsImage(t *testing.T) {
	c := qt.New(t)

	c.Assert(mimekit.IsImage("image/png"), qt.IsTrue)
	c.Assert(mimekit.IsImage("image/jpeg"), qt.IsTrue)
	c.Assert(mimekit.IsImage("application/pdf"), qt.IsFalse)
	c.Assert(mimekit.IsImage("text/plain"), qt.IsFalse)
}

func TestIsDoc(t *testing.T) {
	c := qt.New(t)

	c.Assert(mimekit.IsDoc("application/pdf"), qt.IsTrue)
	c.Assert(mimekit.IsDoc("image/jpeg"), qt.IsTrue)
	c.Assert(mimekit.IsDoc("text/plain"), qt.IsFalse)
}

func TestIsInlineSafe(t *testing.T) {
	c := qt.New(t)

	// Renderable, non-active content → safe to serve inline.
	c.Assert(mimekit.IsInlineSafe("image/png"), qt.IsTrue)
	c.Assert(mimekit.IsInlineSafe("image/jpeg"), qt.IsTrue)
	c.Assert(mimekit.IsInlineSafe("image/gif"), qt.IsTrue)
	c.Assert(mimekit.IsInlineSafe("image/webp"), qt.IsTrue)
	c.Assert(mimekit.IsInlineSafe("application/pdf"), qt.IsTrue)
	c.Assert(mimekit.IsInlineSafe("text/plain"), qt.IsTrue)

	// Active content MUST NOT be inline-able — it would execute in our
	// origin (stored-XSS). These are the security-critical assertions.
	c.Assert(mimekit.IsInlineSafe("text/html"), qt.IsFalse)
	c.Assert(mimekit.IsInlineSafe("image/svg+xml"), qt.IsFalse)
	c.Assert(mimekit.IsInlineSafe("application/xhtml+xml"), qt.IsFalse)

	// Opaque / binary types fall back to download.
	c.Assert(mimekit.IsInlineSafe("application/zip"), qt.IsFalse)
	c.Assert(mimekit.IsInlineSafe("application/octet-stream"), qt.IsFalse)
	c.Assert(mimekit.IsInlineSafe(""), qt.IsFalse)

	// Normalisation: parameters are stripped and case is folded, so a
	// stored value with a charset or odd casing still resolves correctly.
	c.Assert(mimekit.IsInlineSafe("text/plain; charset=utf-8"), qt.IsTrue)
	c.Assert(mimekit.IsInlineSafe("IMAGE/PNG"), qt.IsTrue)
	c.Assert(mimekit.IsInlineSafe("application/pdf; qs=0.001"), qt.IsTrue)
	// Active content stays excluded even with parameters.
	c.Assert(mimekit.IsInlineSafe("text/html; charset=utf-8"), qt.IsFalse)
	c.Assert(mimekit.IsInlineSafe("image/svg+xml; charset=utf-8"), qt.IsFalse)
}

func TestExtensionByMime(t *testing.T) {
	c := qt.New(t)

	c.Assert(mimekit.ExtensionByMime("image/png"), qt.Equals, ".png")
	c.Assert(mimekit.ExtensionByMime("image/jpeg"), qt.Equals, ".jpg")
	c.Assert(mimekit.ExtensionByMime("application/pdf"), qt.Equals, ".pdf")
	c.Assert(mimekit.ExtensionByMime("text/plain"), qt.Equals, ".txt")
	c.Assert(mimekit.ExtensionByMime("application/unknown"), qt.Equals, ".unknown")
}

func TestImageContentTypes(t *testing.T) {
	c := qt.New(t)

	expected := []string{"image/gif", "image/jpeg", "image/png", "image/webp"}
	c.Assert(mimekit.ImageContentTypes(), qt.DeepEquals, expected)
}

func TestDocContentTypes(t *testing.T) {
	c := qt.New(t)

	expected := []string{"image/gif", "image/jpeg", "image/png", "image/webp", "application/pdf"}
	c.Assert(mimekit.DocContentTypes(), qt.DeepEquals, expected)
}

func TestXMLContentTypes(t *testing.T) {
	c := qt.New(t)

	expected := []string{"application/xml", "text/xml"}
	c.Assert(mimekit.XMLContentTypes(), qt.DeepEquals, expected)
}

func TestFormatContentDisposition(t *testing.T) {
	testCases := []struct {
		name     string
		filename string
		expected string
	}{
		// Normal filenames
		{"Simple", "filename.txt", `attachment; filename=filename.txt`},
		{"With spaces", "my document.pdf", `attachment; filename="my document.pdf"`},
		{"With numbers", "report_2023.xlsx", `attachment; filename=report_2023.xlsx`},

		// Exotic filenames
		{"Unicode characters", "résumé.pdf", `attachment; filename*=utf-8''r%C3%A9sum%C3%A9.pdf`},
		{"Emojis", "😀document.txt", `attachment; filename*=utf-8''%F0%9F%98%80document.txt`},
		{"Chinese characters", "文件.doc", `attachment; filename*=utf-8''%E6%96%87%E4%BB%B6.doc`},
		{"Arabic characters", "ملف.pdf", `attachment; filename*=utf-8''%D9%85%D9%84%D9%81.pdf`},
		{"Mixed scripts", "file名称.txt", `attachment; filename*=utf-8''file%E5%90%8D%E7%A7%B0.txt`},

		// Potentially problematic filenames
		{"With quotes", `"quoted".pdf`, `attachment; filename="\"quoted\".pdf"`},
		{"Backslashes", `back\\slash.txt`, `attachment; filename="back\\\\slash.txt"`},
		{"Path traversal attempt", "../../../etc/passwd", `attachment; filename="../../../etc/passwd"`},
		{"With newlines", "new\nline.txt", `attachment; filename*=utf-8''new%0Aline.txt`},
		{"With tabs", "tab\tchar.txt", `attachment; filename="tab	char.txt"`},
		{"Very long", "a" + strings.Repeat("b", 100) + ".txt", `attachment; filename=a` + strings.Repeat("b", 100) + `.txt`},
		{"With control chars", "control\u0001char.txt", `attachment; filename*=utf-8''control%01char.txt`},
		{"HTML injection attempt", "<script>alert('xss')</script>.txt", `attachment; filename="<script>alert('xss')</script>.txt"`},
		{"Empty filename", "", `attachment; filename=""`},
		{"Only extension", ".config", `attachment; filename=.config`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			c.Assert(mimekit.FormatContentDisposition(tc.filename), qt.Equals, tc.expected)
		})
	}
}

func TestFormatInlineContentDisposition(t *testing.T) {
	testCases := []struct {
		name     string
		filename string
		expected string
	}{
		{"Simple", "photo.jpg", `inline; filename=photo.jpg`},
		{"With spaces", "my document.pdf", `inline; filename="my document.pdf"`},
		{"Unicode characters", "résumé.pdf", `inline; filename*=utf-8''r%C3%A9sum%C3%A9.pdf`},
		{"Empty filename", "", `inline; filename=""`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			c.Assert(mimekit.FormatInlineContentDisposition(tc.filename), qt.Equals, tc.expected)
		})
	}
}
