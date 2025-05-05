package mimekit_test

import (
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
	c.Assert(mimekit.IsDoc("image/jpeg"), qt.IsFalse)
	c.Assert(mimekit.IsDoc("text/plain"), qt.IsFalse)
}

func TestExtensionByMime(t *testing.T) {
	c := qt.New(t)

	c.Assert(mimekit.ExtensionByMime("image/png"), qt.Equals, ".png")
	c.Assert(mimekit.ExtensionByMime("image/jpeg"), qt.Equals, ".jpg")
	c.Assert(mimekit.ExtensionByMime("application/pdf"), qt.Equals, ".pdf")
	c.Assert(mimekit.ExtensionByMime("text/plain"), qt.Equals, ".txt")
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
