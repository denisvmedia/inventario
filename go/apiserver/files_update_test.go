package apiserver_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/models"
)

// seedCommodityFile creates a file row attached to the first fixture commodity
// with the given on-disk path, returning the created entity.
func seedCommodityFile(c *qt.C, params apiserver.Params, user *models.User, path string) *models.FileEntity {
	registrySet := getRegistrySetFromParams(params, user)
	commodities := must.Must(registrySet.CommodityRegistry.List(context.Background()))
	c.Assert(len(commodities) > 0, qt.IsTrue, qt.Commentf("fixture must have ≥1 commodity"))

	return must.Must(registrySet.FileRegistry.Create(context.Background(), models.FileEntity{
		Type:     models.FileTypeImage,
		Category: models.FileCategoryImages,
		Links:    []models.FileLink{{EntityType: "commodity", EntityID: commodities[0].ID, Meta: "images"}},
		File: &models.File{
			Path:         path,
			OriginalPath: path + ".jpg",
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	}))
}

func putFile(params apiserver.Params, user *models.User, slug, fileID, body string) *httptest.ResponseRecorder {
	req := must.Must(http.NewRequest(
		http.MethodPut,
		"/api/v1/g/"+slug+"/files/"+fileID,
		bytes.NewReader([]byte(body)),
	))
	req.Header.Set("Content-Type", "application/json")
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	apiserver.APIServer(params, &mockRestoreWorker{}).ServeHTTP(rr, req)
	return rr
}

// TestUpdateFile_PutWithoutPathPreservesPath is the regression for #2033's
// secondary bug: a PUT that edits only the title carries NO `path`, and the
// handler must keep the persisted path rather than blank it with
// CleanFilename("") = "".
func TestUpdateFile_PutWithoutPathPreservesPath(t *testing.T) {
	c := qt.New(t)

	params, user, group := newParams()
	file := seedCommodityFile(c, params, user, "original-name")

	body := `{"data":{"id":"` + file.ID + `","type":"files","attributes":{"title":"Receipt"}}}`
	rr := putFile(params, user, group.Slug, file.ID, body)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	// Path must NOT have been blanked.
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.attributes.path"), "original-name")
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.attributes.title"), "Receipt")

	// And persisted state confirms it.
	registrySet := getRegistrySetFromParams(params, user)
	fresh := must.Must(registrySet.FileRegistry.Get(context.Background(), file.ID))
	c.Check(fresh.Path, qt.Equals, "original-name")
}

// TestUpdateFile_PutDoesNotChangeLinks pins that attachments are not part of
// the PUT body: link fields a client still sends are ignored, and the file
// keeps exactly the links it had. Attaching and detaching go through
// /files/{id}/links.
func TestUpdateFile_PutDoesNotChangeLinks(t *testing.T) {
	c := qt.New(t)

	params, user, group := newParams()
	file := seedCommodityFile(c, params, user, "original-name")
	c.Assert(file.Links, qt.HasLen, 1)

	body := `{"data":{"id":"` + file.ID + `","type":"files","attributes":{` +
		`"title":"Receipt","linked_entity_type":"location","linked_entity_id":"somewhere","linked_entity_meta":"files"}}}`
	rr := putFile(params, user, group.Slug, file.ID, body)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	registrySet := getRegistrySetFromParams(params, user)
	fresh := must.Must(registrySet.FileRegistry.Get(context.Background(), file.ID))
	c.Assert(fresh.Links, qt.HasLen, 1)
	c.Check(fresh.Links[0].EntityType, qt.Equals, "commodity")
	c.Check(fresh.Links[0].EntityID, qt.Equals, file.Links[0].EntityID)
	c.Check(fresh.Links[0].Meta, qt.Equals, "images")
}

// TestUpdateFile_NewPathOverwrites confirms a non-empty `path` still renames.
func TestUpdateFile_NewPathOverwrites(t *testing.T) {
	c := qt.New(t)

	params, user, group := newParams()
	file := seedCommodityFile(c, params, user, "original-name")

	body := `{"data":{"id":"` + file.ID + `","type":"files","attributes":{"path":"renamed"}}}`
	rr := putFile(params, user, group.Slug, file.ID, body)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.attributes.path"), "renamed")
}

// TestUpdateFile_RejectsOverlongTitle proves the nested attribute validator
// runs end-to-end (#2033): an invalid attribute is rejected with 422 instead of
// silently persisting.
func TestUpdateFile_RejectsOverlongTitle(t *testing.T) {
	c := qt.New(t)

	params, user, group := newParams()
	file := seedCommodityFile(c, params, user, "original-name")

	body := `{"data":{"id":"` + file.ID + `","type":"files","attributes":{"title":"` + strings.Repeat("t", 256) + `"}}}`
	rr := putFile(params, user, group.Slug, file.ID, body)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}
