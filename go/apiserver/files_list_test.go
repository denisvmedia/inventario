package apiserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/models"
)

// The Files page is a filter surface, and a filter that silently drops a
// clause answers with the wrong set rather than an error — the user reads it
// as "nothing matched". #2114 N2 flagged listFiles, deleteFile and
// listCategoryCounts at 0%.

type filesListEnv struct {
	handler http.Handler
	params  apiserver.Params
	token   string
	slug    string
	byTitle map[string]string
}

func newFilesListEnv(c *qt.C) *filesListEnv {
	c.Helper()

	params, owner, group := newParams()
	ownerCtx := createTestUserContextWithGroup(owner.ID, owner.TenantID, group.ID)
	set := must.Must(params.FactorySet.CreateUserRegistrySet(ownerCtx))

	seed := []struct {
		title    string
		category models.FileCategory
		fileType models.FileType
		tags     []string
		ext      string
		mime     string
	}{
		{"invoice-2026", models.FileCategoryDocuments, models.FileTypeDocument, []string{"invoice", "tax"}, ".pdf", "application/pdf"},
		{"manual-drill", models.FileCategoryDocuments, models.FileTypeDocument, []string{"manual"}, ".pdf", "application/pdf"},
		{"photo-shelf", models.FileCategoryImages, models.FileTypeImage, []string{"workshop"}, ".jpg", "image/jpeg"},
	}

	byTitle := make(map[string]string, len(seed))
	for _, s := range seed {
		f := must.Must(set.FileRegistry.Create(ownerCtx, models.FileEntity{
			Title:    s.title,
			Type:     s.fileType,
			Category: s.category,
			Tags:     s.tags,
			File: &models.File{
				Path:         s.title,
				OriginalPath: s.title + s.ext,
				Ext:          s.ext,
				MIMEType:     s.mime,
			},
		}))
		byTitle[s.title] = f.ID
	}

	return &filesListEnv{
		handler: apiserver.APIServer(params, &mockRestoreWorker{}),
		params:  params,
		token:   createTestJWTToken(owner.ID),
		slug:    group.Slug,
		byTitle: byTitle,
	}
}

// titles reads the returned file titles. The shared fixture already seeds a
// library, so asserting on which files came back is both stronger than a
// count and immune to that library changing size.
func (e *filesListEnv) titles(c *qt.C, query string) []string {
	c.Helper()

	rr := e.get(c, query)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var body struct {
		Data []struct {
			Title string `json:"title"`
		} `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)

	out := make([]string, 0, len(body.Data))
	for _, f := range body.Data {
		out = append(out, f.Title)
	}
	sort.Strings(out)
	return out
}

func (e *filesListEnv) total(c *qt.C, query string) int {
	c.Helper()

	rr := e.get(c, query)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var body struct {
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
	return body.Meta.Total
}

func (e *filesListEnv) get(c *qt.C, query string) *httptest.ResponseRecorder {
	c.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/g/"+e.slug+"/files"+query, nil)
	req.Header.Set("Authorization", "Bearer "+e.token)
	rr := httptest.NewRecorder()
	e.handler.ServeHTTP(rr, req)
	return rr
}

func TestFilesList_ReturnsEverythingUnfiltered(t *testing.T) {
	c := qt.New(t)
	env := newFilesListEnv(c)

	got := env.titles(c, "?limit=100")
	for title := range env.byTitle {
		c.Check(got, qt.Contains, title)
	}
}

func TestFilesList_FiltersByCategoryAndType(t *testing.T) {
	c := qt.New(t)
	env := newFilesListEnv(c)

	images := env.titles(c, "?category=images&limit=100")
	c.Check(images, qt.Contains, "photo-shelf")
	c.Check(images, qt.Not(qt.Contains), "invoice-2026")

	documents := env.titles(c, "?type=document&limit=100")
	c.Check(documents, qt.Contains, "invoice-2026")
	c.Check(documents, qt.Contains, "manual-drill")
	c.Check(documents, qt.Not(qt.Contains), "photo-shelf")

	rr := env.get(c, "?category=not-a-category")
	c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)
}

// `?tags=a,b` and a repeated `?tag=a&tag=b` are both public surface and mean
// the same thing. The FE toolbar sends the first for multi-pill selection and
// a tag chip links out with the second, so a client that picks either must
// get the same answer.
func TestFilesList_TagFilterAcceptsBothShapes(t *testing.T) {
	// Only the seeded files carry tags, so a tag filter answers with exactly
	// the expected titles rather than a count against a shared library.
	//
	// Several tags narrow rather than widen: a file has to carry all of
	// them. invoice-2026 carries invoice and tax; nothing carries both
	// invoice and manual, so that pair is empty where an OR filter would
	// answer with two.
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"comma-separated, one tag", "?tags=invoice", []string{"invoice-2026"}},
		{"singular alias, one tag", "?tag=invoice", []string{"invoice-2026"}},
		{"comma-separated, both tags of one file", "?tags=invoice,tax", []string{"invoice-2026"}},
		{"repeated singular, both tags of one file", "?tag=invoice&tag=tax", []string{"invoice-2026"}},
		{"both shapes at once", "?tags=invoice&tag=tax", []string{"invoice-2026"}},
		{"tags no single file shares", "?tags=invoice,manual", []string{}},
		{"whitespace around a tag", "?tags=%20invoice%20", []string{"invoice-2026"}},
		{"empty entries are ignored", "?tags=,,invoice,,", []string{"invoice-2026"}},
		{"a tag nothing carries", "?tags=nonexistent", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			env := newFilesListEnv(c)

			c.Assert(env.titles(c, tt.query+"&limit=100"), qt.DeepEquals, tt.want)
		})
	}
}

// An empty `?tags=` must not be read as "filter by the empty tag", which
// would answer with nothing and look like an empty library.
func TestFilesList_EmptyTagParamIsNotAFilter(t *testing.T) {
	c := qt.New(t)
	env := newFilesListEnv(c)

	unfiltered := env.total(c, "")
	for _, query := range []string{"?tags=", "?tag=", "?tags=%20", "?tags=&tag="} {
		c.Check(env.total(c, query), qt.Equals, unfiltered,
			qt.Commentf("query %q filtered when it should not have", query))
	}
}

// linked_entity_type and linked_entity_id are meaningless apart. Accepting
// half the pair would drop the clause and answer with the unfiltered set,
// which reads as success.
func TestFilesList_LinkedEntityPairIsBothOrNeither(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  int
	}{
		{"neither", "", http.StatusOK},
		{"type only", "?linked_entity_type=commodity", http.StatusBadRequest},
		{"id only", "?linked_entity_id=c1", http.StatusBadRequest},
		{"both", "?linked_entity_type=commodity&linked_entity_id=c1", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			env := newFilesListEnv(c)

			rr := env.get(c, tt.query)
			c.Assert(rr.Code, qt.Equals, tt.want)
		})
	}
}

func TestFilesList_Paginates(t *testing.T) {
	c := qt.New(t)
	env := newFilesListEnv(c)

	total := env.total(c, "?limit=100")
	c.Assert(total > 2, qt.IsTrue)

	first := env.titles(c, "?limit=2&page=1")
	c.Assert(first, qt.HasLen, 2)

	second := env.titles(c, "?limit=2&page=2")
	c.Assert(second, qt.HasLen, 2)

	// Pages do not overlap, and the total stays the library rather than the
	// page — the FE draws its pager from that number.
	for _, title := range second {
		c.Check(first, qt.Not(qt.Contains), title)
	}
	c.Check(env.total(c, "?limit=2&page=2"), qt.Equals, total)
}

// A nonsense page or limit falls back to the default rather than answering
// with an empty list or a 500.
func TestFilesList_IgnoresUnusablePagination(t *testing.T) {
	c := qt.New(t)
	env := newFilesListEnv(c)

	fallback := env.titles(c, "")
	for _, query := range []string{"?page=0", "?page=-1", "?page=abc", "?limit=0", "?limit=-5", "?limit=abc", "?limit=1000"} {
		c.Check(env.titles(c, query), qt.DeepEquals, fallback,
			qt.Commentf("query %q changed the result set", query))
	}
}

func TestFilesCategoryCounts(t *testing.T) {
	c := qt.New(t)
	env := newFilesListEnv(c)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/g/"+env.slug+"/files/category-counts", nil)
	req.Header.Set("Authorization", "Bearer "+env.token)
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var counts struct {
		Data struct {
			Images    int `json:"images"`
			Documents int `json:"documents"`
			Other     int `json:"other"`
			All       int `json:"all"`
		} `json:"data"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &counts), qt.IsNil)

	// The counts are the same library the listing reports, per category.
	c.Check(counts.Data.Images, qt.Equals, env.total(c, "?category=images&limit=100"))
	c.Check(counts.Data.Documents, qt.Equals, env.total(c, "?category=documents&limit=100"))
	c.Check(counts.Data.Images > 0, qt.IsTrue)

	// `all` is the sum, and the stats bar reads it directly — a drifting
	// total there is a number nobody can reconcile against the list.
	c.Check(counts.Data.All, qt.Equals, counts.Data.Images+counts.Data.Documents+counts.Data.Other)
}

// The counts honor the same filters as the listing, so the stats bar
// describes what the user is looking at rather than the whole library.
func TestFilesCategoryCounts_HonorsTheTagFilter(t *testing.T) {
	c := qt.New(t)
	env := newFilesListEnv(c)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/g/"+env.slug+"/files/category-counts?tags=invoice", nil)
	req.Header.Set("Authorization", "Bearer "+env.token)
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var counts struct {
		Data struct {
			Documents int `json:"documents"`
			All       int `json:"all"`
		} `json:"data"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &counts), qt.IsNil)
	c.Check(counts.Data.All, qt.Equals, 1)
	c.Check(counts.Data.Documents, qt.Equals, 1)
}

func TestFilesDelete(t *testing.T) {
	c := qt.New(t)
	env := newFilesListEnv(c)

	id := env.byTitle["invoice-2026"]
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/g/"+env.slug+"/files/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+env.token)
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)

	// Gone from the listing, not just from the detail route.
	c.Check(env.titles(c, "?limit=100"), qt.Not(qt.Contains), "invoice-2026")
}

func TestFilesDelete_UnknownID(t *testing.T) {
	c := qt.New(t)
	env := newFilesListEnv(c)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/g/"+env.slug+"/files/no-such-file", nil)
	req.Header.Set("Authorization", "Bearer "+env.token)
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

// The chip counts on a commodity's Files tab come from this endpoint and sit
// directly above that commodity's file list. Without the linked-entity pair
// they would count the whole group, which is a number the list underneath
// cannot account for.
func TestFilesCategoryCounts_ScopesToTheLinkedEntity(t *testing.T) {
	c := qt.New(t)

	params, owner, group := newParams()
	ownerCtx := createTestUserContextWithGroup(owner.ID, owner.TenantID, group.ID)
	set := must.Must(params.FactorySet.CreateUserRegistrySet(ownerCtx))

	seed := []struct {
		title       string
		category    models.FileCategory
		fileType    models.FileType
		mime        string
		ext         string
		commodityID string
	}{
		{"a-photo", models.FileCategoryImages, models.FileTypeImage, "image/jpeg", ".jpg", "com-a"},
		{"a-manual", models.FileCategoryDocuments, models.FileTypeDocument, "application/pdf", ".pdf", "com-a"},
		{"b-photo-1", models.FileCategoryImages, models.FileTypeImage, "image/jpeg", ".jpg", "com-b"},
		{"b-photo-2", models.FileCategoryImages, models.FileTypeImage, "image/png", ".png", "com-b"},
	}
	for _, s := range seed {
		must.Must(set.FileRegistry.Create(ownerCtx, models.FileEntity{
			Title:            s.title,
			Type:             s.fileType,
			Category:         s.category,
			LinkedEntityType: "commodity",
			LinkedEntityID:   s.commodityID,
			File: &models.File{
				Path:         s.title,
				OriginalPath: s.title + s.ext,
				Ext:          s.ext,
				MIMEType:     s.mime,
			},
		}))
	}

	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	token := createTestJWTToken(owner.ID)

	type bucket struct {
		Images    int `json:"images"`
		Documents int `json:"documents"`
		Other     int `json:"other"`
		All       int `json:"all"`
	}
	counts := func(c *qt.C, query string) (bucket, int) {
		c.Helper()
		req := httptest.NewRequest(http.MethodGet,
			"/api/v1/g/"+group.Slug+"/files/category-counts"+query, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			return bucket{}, rr.Code
		}
		var body struct {
			Data bucket `json:"data"`
		}
		c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
		return body.Data, rr.Code
	}

	comA, status := counts(c, "?linked_entity_type=commodity&linked_entity_id=com-a")
	c.Assert(status, qt.Equals, http.StatusOK)
	c.Check(comA.Images, qt.Equals, 1)
	c.Check(comA.Documents, qt.Equals, 1)
	c.Check(comA.Other, qt.Equals, 0)
	c.Check(comA.All, qt.Equals, 2)

	comB, status := counts(c, "?linked_entity_type=commodity&linked_entity_id=com-b")
	c.Assert(status, qt.Equals, http.StatusOK)
	c.Check(comB.Images, qt.Equals, 2)
	c.Check(comB.Documents, qt.Equals, 0)
	c.Check(comB.Other, qt.Equals, 0)
	c.Check(comB.All, qt.Equals, 2)

	// No pair is the Files page's own view: the whole group, which holds
	// unlinked files too. Asserted against the scoped answers rather than a
	// literal, so the fixture can grow without rewriting the expectation.
	all, status := counts(c, "")
	c.Assert(status, qt.Equals, http.StatusOK)
	c.Check(all.All, qt.Equals, all.Images+all.Documents+all.Other)
	c.Check(all.Images >= comA.Images+comB.Images, qt.IsTrue)
	c.Check(all.Documents >= comA.Documents+comB.Documents, qt.IsTrue)
	c.Check(all.All > comA.All+comB.All, qt.IsTrue,
		qt.Commentf("the group holds more than these two commodities' files"))

	// Half a pair is a filter the caller believes is applied and is not, so
	// it is rejected here the same way GET /files rejects it.
	_, status = counts(c, "?linked_entity_type=commodity")
	c.Check(status, qt.Equals, http.StatusBadRequest)
	_, status = counts(c, "?linked_entity_id=com-a")
	c.Check(status, qt.Equals, http.StatusBadRequest)
}
