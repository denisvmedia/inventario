package apiserver_test

import (
	"context"
	"net/http"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/models"
)

// DELETE /files/{id}?unless_linked=true trashes a file only while nothing
// links to it, and tells trashed (204), kept (409 file.linked) and gone (404)
// apart (#2930).
func TestFileDelete_UnlessLinked(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	ids := env.commodityIDs()
	c.Assert(ids, qt.Not(qt.HasLen), 0)

	unused := env.bulkFile("unused-upload")
	rr := env.do(http.MethodDelete, "/files/"+unused.ID+"?unless_linked=true", "")
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodGet, "/trash/files", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data[0].id"), unused.ID)

	// In Trash already, and never there at all: both gone.
	rr = env.do(http.MethodDelete, "/files/"+unused.ID+"?unless_linked=true", "")
	c.Check(rr.Code, qt.Equals, http.StatusNotFound, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodDelete, "/files/no-such-file?unless_linked=true", "")
	c.Check(rr.Code, qt.Equals, http.StatusNotFound, qt.Commentf("body=%s", rr.Body.String()))

	// A form reads the file, finds no link and asks to trash it; in between,
	// another tab attached it to an item. The file stays with the item.
	raced := env.bulkFile("raced-upload")
	rr = env.do(http.MethodGet, "/files/"+raced.ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Check(rr.Body.String(), checkers.JSONPathMatches("$.attributes.links", qt.HasLen), 0)
	rr = env.do(http.MethodPost, "/files/"+raced.ID+"/links",
		`{"data":{"type":"file-links","attributes":{"entity_type":"commodity","entity_id":"`+ids[0]+`","meta":"images"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodDelete, "/files/"+raced.ID+"?unless_linked=true", "")
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].code"), "file.linked")
	rr = env.do(http.MethodGet, "/files/"+raced.ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("the linked file must stay live"))

	// Without the parameter the file goes to Trash whatever links it.
	rr = env.do(http.MethodDelete, "/files/"+raced.ID+"?unless_linked=false", "")
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent, qt.Commentf("body=%s", rr.Body.String()))
}

// An export bundle is held by its export link: unless_linked keeps it,
// where a plain DELETE removes it for good.
func TestFileDelete_UnlessLinkedKeepsAnExportBundle(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)

	ctx := createTestUserContextWithGroup(env.user.ID, env.user.TenantID, env.group.ID)
	set := must.Must(env.params.FactorySet.CreateUserRegistrySet(ctx))
	bundle := must.Must(set.FileRegistry.Create(ctx, models.FileEntity{
		Title:    "backup",
		Type:     models.FileTypeOther,
		Category: models.FileCategoryOther,
		Links:    []models.FileLink{{EntityType: models.FileLinkExport, EntityID: "export-1", Meta: "xml-1.0"}},
		File: &models.File{
			Path:         "backup",
			OriginalPath: "blobs/backup.xml",
			Ext:          ".xml",
			MIMEType:     "application/xml",
		},
	}))

	rr := env.do(http.MethodDelete, "/files/"+bundle.ID+"?unless_linked=true", "")
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].code"), "file.linked")
	_, err := set.FileRegistry.Get(context.Background(), bundle.ID)
	c.Check(err, qt.IsNil)
}

func TestFileDelete_UnlessLinkedRefusesANonBoolean(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	file := env.bulkFile("upload")

	rr := env.do(http.MethodDelete, "/files/"+file.ID+"?unless_linked=maybe", "")
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodGet, "/files/"+file.ID, "")
	c.Check(rr.Code, qt.Equals, http.StatusOK)
}

// A viewer may not trash a file, with or without unless_linked.
func TestFileDelete_UnlessLinkedNeedsAnEditor(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	file := env.bulkFile("upload")
	env.setRole(models.GroupRoleViewer)

	rr := env.do(http.MethodDelete, "/files/"+file.ID+"?unless_linked=true", "")
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf("body=%s", rr.Body.String()))
}
