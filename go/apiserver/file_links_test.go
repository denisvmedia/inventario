package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/models"
)

// fileLinksEnv is a group with two fixture items and one receipt attached to
// the first of them.
type fileLinksEnv struct {
	params     apiserver.Params
	user       *models.User
	slug       string
	first      *models.Commodity
	second     *models.Commodity
	receiptID  string
	locationID string
}

func newFileLinksEnv(c *qt.C) fileLinksEnv {
	c.Helper()
	params, user, group := newParams()
	set := getRegistrySetFromParams(params, user)
	commodities := must.Must(set.CommodityRegistry.List(context.Background()))
	c.Assert(len(commodities) >= 2, qt.IsTrue, qt.Commentf("fixture must have two commodities"))
	locations := must.Must(set.LocationRegistry.List(context.Background()))
	c.Assert(locations, qt.Not(qt.HasLen), 0)

	receipt := must.Must(set.FileRegistry.Create(context.Background(), models.FileEntity{
		Title:    "receipt",
		Type:     models.FileTypeDocument,
		Category: models.FileCategoryDocuments,
		Links:    []models.FileLink{{EntityType: models.FileLinkCommodity, EntityID: commodities[0].ID, Meta: models.FileLinkMetaDocuments}},
		File: &models.File{
			Path:         "receipt",
			OriginalPath: "receipt.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
			SHA256:       "same-content",
		},
	}))
	return fileLinksEnv{
		params:     params,
		user:       user,
		slug:       group.Slug,
		first:      commodities[0],
		second:     commodities[1],
		receiptID:  receipt.ID,
		locationID: locations[0].ID,
	}
}

func (e fileLinksEnv) do(c *qt.C, method, path, body string) *httptest.ResponseRecorder {
	c.Helper()
	req := must.Must(http.NewRequest(method, "/api/v1/g/"+e.slug+path, bytes.NewReader([]byte(body))))
	req.Header.Set("Content-Type", "application/json")
	addTestUserAuthHeader(req, e.user.ID)
	rr := httptest.NewRecorder()
	apiserver.APIServer(e.params, &mockRestoreWorker{}).ServeHTTP(rr, req)
	return rr
}

func (e fileLinksEnv) attach(c *qt.C, fileID, entityType, entityID, meta string) *httptest.ResponseRecorder {
	c.Helper()
	attrs := map[string]string{"entity_type": entityType, "entity_id": entityID}
	if meta != "" {
		attrs["meta"] = meta
	}
	body := must.Must(json.Marshal(map[string]any{
		"data": map[string]any{"type": "file-links", "attributes": attrs},
	}))
	return e.do(c, http.MethodPost, "/files/"+fileID+"/links", string(body))
}

type linkView struct {
	ID         string `json:"id"`
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	EntityName string `json:"entity_name"`
	Meta       string `json:"meta"`
}

func linksOf(c *qt.C, rr *httptest.ResponseRecorder) []linkView {
	c.Helper()
	var body struct {
		Attributes struct {
			Links []linkView `json:"links"`
		} `json:"attributes"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil, qt.Commentf("body: %s", rr.Body.String()))
	return body.Attributes.Links
}

func TestFileLinksAPI_AttachIsIdempotent(t *testing.T) {
	c := qt.New(t)
	e := newFileLinksEnv(c)

	rr := e.attach(c, e.receiptID, "commodity", e.second.ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body: %s", rr.Body.String()))
	links := linksOf(c, rr)
	c.Assert(links, qt.HasLen, 2)
	var added linkView
	for _, l := range links {
		if l.EntityID == e.second.ID {
			added = l
		}
	}
	c.Check(added.EntityName, qt.Equals, e.second.Name)
	// No meta given: the item's default bucket.
	c.Check(added.Meta, qt.Equals, models.FileLinkMetaDocuments)

	rr = e.attach(c, e.receiptID, "commodity", e.second.ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Check(linksOf(c, rr), qt.HasLen, 2)
}

func TestFileLinksAPI_AttachRejectsBadTargets(t *testing.T) {
	c := qt.New(t)
	e := newFileLinksEnv(c)

	tests := []struct {
		name       string
		entityType string
		entityID   string
		meta       string
	}{
		{name: "unknown item", entityType: "commodity", entityID: "no-such-item"},
		{name: "unknown entity type", entityType: "export", entityID: e.second.ID},
		{name: "bucket of another entity type", entityType: "location", entityID: e.locationID, meta: "invoices"},
		{name: "missing id", entityType: "commodity"},
	}
	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			rr := e.attach(c, e.receiptID, tt.entityType, tt.entityID, tt.meta)
			c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body: %s", rr.Body.String()))
		})
	}

	rr := e.attach(c, "no-such-file", "commodity", e.second.ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestFileLinksAPI_DetachKeepsTheFile(t *testing.T) {
	c := qt.New(t)
	e := newFileLinksEnv(c)

	rr := e.do(c, http.MethodGet, "/files/"+e.receiptID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	links := linksOf(c, rr)
	c.Assert(links, qt.HasLen, 1)
	c.Check(links[0].EntityName, qt.Equals, e.first.Name)

	rr = e.do(c, http.MethodDelete, "/files/"+e.receiptID+"/links/"+links[0].ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", rr.Body.String()))
	c.Check(linksOf(c, rr), qt.HasLen, 0)

	// The file is still there, as a standalone file.
	rr = e.do(c, http.MethodGet, "/files/"+e.receiptID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Check(linksOf(c, rr), qt.HasLen, 0)

	rr = e.do(c, http.MethodDelete, "/files/"+e.receiptID+"/links/"+links[0].ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestFileLinksAPI_Duplicates(t *testing.T) {
	c := qt.New(t)
	e := newFileLinksEnv(c)
	set := getRegistrySetFromParams(e.params, e.user)
	copyOf := must.Must(set.FileRegistry.Create(context.Background(), models.FileEntity{
		Title:    "receipt again",
		Type:     models.FileTypeDocument,
		Category: models.FileCategoryDocuments,
		File: &models.File{
			Path:         "receipt-again",
			OriginalPath: "receipt-again.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
			SHA256:       "same-content",
		},
	}))

	rr := e.do(c, http.MethodGet, "/files/"+copyOf.ID+"/duplicates", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", rr.Body.String()))
	var body struct {
		Data []struct {
			ID    string     `json:"id"`
			Links []linkView `json:"links"`
		} `json:"data"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
	c.Assert(body.Data, qt.HasLen, 1)
	c.Check(body.Data[0].ID, qt.Equals, e.receiptID)
	c.Assert(body.Data[0].Links, qt.HasLen, 1)
	c.Check(body.Data[0].Links[0].EntityName, qt.Equals, e.first.Name)
}

func TestFileLinksAPI_ListNamesTheLinkedEntities(t *testing.T) {
	c := qt.New(t)
	e := newFileLinksEnv(c)

	rr := e.do(c, http.MethodGet, "/files?linked_entity_type=commodity&linked_entity_id="+e.first.ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	var body struct {
		Data []struct {
			ID    string     `json:"id"`
			Links []linkView `json:"links"`
		} `json:"data"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
	var found bool
	for _, f := range body.Data {
		if f.ID != e.receiptID {
			continue
		}
		found = true
		c.Assert(f.Links, qt.HasLen, 1)
		c.Check(f.Links[0].EntityName, qt.Equals, e.first.Name)
	}
	c.Assert(found, qt.IsTrue)
}
