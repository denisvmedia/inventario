package apiserver_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/checkers"
	"github.com/denisvmedia/inventario/models"
)

// seedCommodityFile creates a file row attached to the first fixture commodity
// with the given on-disk path, returning the created entity.
func seedCommodityFile(c *qt.C, params apiserver.Params, user *models.User, path string) *models.FileEntity {
	registrySet := getRegistrySetFromParams(params, user)
	commodities := must.Must(registrySet.CommodityRegistry.List(context.Background()))
	c.Assert(len(commodities) > 0, qt.IsTrue, qt.Commentf("fixture must have ≥1 commodity"))

	return must.Must(registrySet.FileRegistry.Create(context.Background(), models.FileEntity{
		Type:             models.FileTypeImage,
		Category:         models.FileCategoryImages,
		LinkedEntityType: "commodity",
		LinkedEntityID:   commodities[0].ID,
		LinkedEntityMeta: "images",
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

// TestUpdateFile_LinkOnlyPutPreservesPath is the core regression for #2033's
// secondary bug: the live commodity-attach flow (PR #2032) PUTs with NO `path`,
// and the handler used to blank the persisted path with CleanFilename("") = "".
// After the fix the path is preserved and the request still succeeds.
func TestUpdateFile_LinkOnlyPutPreservesPath(t *testing.T) {
	c := qt.New(t)

	params, user, group := newParams()
	file := seedCommodityFile(c, params, user, "original-name")

	// Mirror frontend/src/features/commodities/draft.ts::uploadPendingFiles:
	// link-only PUT, no `path`, no `linked_entity_meta`.
	body := `{"data":{"id":"` + file.ID + `","type":"files","attributes":{` +
		`"linked_entity_type":"commodity","linked_entity_id":"` + file.LinkedEntityID + `"}}}`
	rr := putFile(params, user, group.Slug, file.ID, body)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	// Path must NOT have been blanked.
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.attributes.path"), "original-name")

	// And persisted state confirms it.
	registrySet := getRegistrySetFromParams(params, user)
	fresh := must.Must(registrySet.FileRegistry.Get(context.Background(), file.ID))
	c.Check(fresh.Path, qt.Equals, "original-name")
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

// TestUpdateFile_RejectsBogusLinkedEntityMeta proves the previously-dead nested
// validator now runs end-to-end: a non-empty invalid `linked_entity_meta` is
// rejected with 422 instead of silently persisting.
func TestUpdateFile_RejectsBogusLinkedEntityMeta(t *testing.T) {
	c := qt.New(t)

	params, user, group := newParams()
	file := seedCommodityFile(c, params, user, "original-name")

	body := `{"data":{"id":"` + file.ID + `","type":"files","attributes":{` +
		`"linked_entity_type":"commodity","linked_entity_id":"` + file.LinkedEntityID + `",` +
		`"linked_entity_meta":"BOGUS"}}}`
	rr := putFile(params, user, group.Slug, file.ID, body)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}
