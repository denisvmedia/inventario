package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/internal/aivision/mock"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// These drive the real router: the intake endpoint (#2809) takes an API
// token that may write, in a group whose role can edit items, and a body as
// JSON, as a form or as a multipart form with the file.

func newIntakeDeliveryEnv(c *qt.C, mutate ...func(*apiserver.Params)) (*apiTokenEnv, string) {
	c.Helper()
	env := newAPITokenEnv(c, append([]func(*apiserver.Params){func(p *apiserver.Params) {
		p.CommodityScanService = services.NewCommodityScanService(mock.New(), p.FactorySet.CommodityScanAuditRegistry,
			services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20, RateLimitPerHour: 100})
	}}, mutate...)...)
	tok := env.mint(map[string]any{"scope": "read_write", "group_id": env.group.ID})
	return env, tok.Secret
}

// raw sends body with contentType, authenticated by the API token.
func (e *apiTokenEnv) raw(token, path, contentType string, body []byte) *httptest.ResponseRecorder {
	e.c.Helper()
	r := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	e.handler.ServeHTTP(rr, r)
	return rr
}

func paperlessForm(id string) url.Values {
	return url.Values{
		"title":         {"Invoice " + id},
		"source_system": {"paperless-ngx"},
		"document_id":   {id},
		"source_url":    {"https://paperless.example/documents/" + id + "/"},
		"source_name":   {"Paperless-ngx"},
		"seller":        {"Hardware Store"},
		"purchase_date": {"2024-03-14"},
		"amount":        {"EUR129.90"},
	}
}

// multipartDelivery encodes fields and, when content is not nil, the file
// part as Paperless-ngx sends them.
func multipartDelivery(c *qt.C, fields url.Values, partName, fileName string, content []byte) (string, []byte) {
	c.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for name, values := range fields {
		for _, v := range values {
			c.Assert(mw.WriteField(name, v), qt.IsNil)
		}
	}
	if content != nil {
		fw, err := mw.CreateFormFile(partName, fileName)
		c.Assert(err, qt.IsNil)
		_, err = fw.Write(content)
		c.Assert(err, qt.IsNil)
	}
	c.Assert(mw.Close(), qt.IsNil)
	return mw.FormDataContentType(), buf.Bytes()
}

func decodeIntakeDocument(c *qt.C, rr *httptest.ResponseRecorder) jsonapi.IntakeDocumentResponseData {
	c.Helper()
	var resp struct {
		Data jsonapi.IntakeDocumentResponseData `json:"data"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &resp), qt.IsNil, qt.Commentf(rr.Body.String()))
	return resp.Data
}

// groupCtx is the context of the user in the env's group.
func (e *apiTokenEnv) groupCtx() context.Context {
	return appctx.WithGroup(createTestUserContext(e.user.ID, e.user.TenantID), e.group)
}

func (e *apiTokenEnv) groupRegistries() *registry.Set {
	e.c.Helper()
	return must.Must(e.params.FactorySet.CreateUserRegistrySet(e.groupCtx()))
}

func (e *apiTokenEnv) openDrafts() []*models.CommodityDraft {
	e.c.Helper()
	all := must.Must(e.groupRegistries().CommodityDraftRegistry.List(e.groupCtx()))
	var open []*models.CommodityDraft
	for _, d := range all {
		if d.Status == models.CommodityDraftOpen {
			open = append(open, d)
		}
	}
	return open
}

// pdf is enough of a PDF for the content sniffer.
var pdf = []byte("%PDF-1.4\n1 0 obj << /Type /Catalog >> endobj\ntrailer << /Root 1 0 R >>\n%%EOF\n")

func TestIntakeDelivery_JSONAndReplay(t *testing.T) {
	c := qt.New(t)
	env, token := newIntakeDeliveryEnv(c)
	path := env.groupPath(env.group, "/intake")

	body := map[string]any{
		"title":         "Invoice 117",
		"source_system": "paperless-ngx",
		"document_id":   117,
		"source_url":    "https://paperless.example/documents/117/",
		"source_name":   "Paperless-ngx",
		"amount":        "129.90",
		"currency":      "EUR",
		"seller":        nil,
	}
	rr := env.withToken(token, http.MethodPost, path, body)
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf(rr.Body.String()))
	doc := decodeIntakeDocument(c, rr)
	c.Assert(doc.Type, qt.Equals, "intake_documents")
	c.Assert(doc.Attributes.State, qt.Equals, models.IntakeDocumentDraft)
	c.Assert(doc.Attributes.ExternalDocumentID, qt.Equals, "117")
	c.Assert(doc.Attributes.DraftID, qt.IsNotNil)
	c.Assert(rr.Body.String(), qt.Not(qt.Contains), "payload_hash")
	c.Assert(env.openDrafts(), qt.HasLen, 1)

	rr = env.withToken(token, http.MethodPost, path, body)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	c.Assert(decodeIntakeDocument(c, rr).ID, qt.Equals, doc.ID)
	c.Assert(env.openDrafts(), qt.HasLen, 1)

	body["title"] = "Invoice 117 corrected"
	rr = env.withToken(token, http.MethodPost, path, body)
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf(rr.Body.String()))
	c.Assert(errorCode(c, rr), qt.Equals, apiserver.IntakeDocumentChangedCode)
	c.Assert(rr.Body.String(), qt.Contains, doc.ID)
	c.Assert(rr.Body.String(), qt.Contains, *doc.Attributes.DraftID)

	// The draft and the item find the document.
	rr = env.withToken(token, http.MethodGet, env.groupPath(env.group, "/intake/documents?draft_id="+*doc.Attributes.DraftID+",other"), nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	var list jsonapi.IntakeDocumentsResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &list), qt.IsNil)
	c.Assert(list.Data, qt.HasLen, 1)
	c.Assert(list.Data[0].ID, qt.Equals, doc.ID)
	c.Assert(list.Data[0].Attributes.State, qt.Equals, models.IntakeDocumentDraft)
	rr = env.withToken(token, http.MethodGet, env.groupPath(env.group, "/intake/documents"), nil)
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf(rr.Body.String()))

	// The rest of the intake routes are still served.
	rr = env.withToken(token, http.MethodGet, env.groupPath(env.group, "/intake/limits"), nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	rr = env.withToken(token, http.MethodGet, env.groupPath(env.group, "/intake/sessions"), nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))

	// The inbox shows the draft with where it came from.
	rr = env.withToken(token, http.MethodGet, env.groupPath(env.group, "/inbox"), nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	c.Assert(rr.Body.String(), qt.Contains, `"source":{"name":"Paperless-ngx","url":"https://paperless.example/documents/117/"}`)
}

func TestIntakeDelivery_Form(t *testing.T) {
	c := qt.New(t)
	env, token := newIntakeDeliveryEnv(c)
	path := env.groupPath(env.group, "/intake")

	rr := env.raw(token, path, "application/x-www-form-urlencoded", []byte(paperlessForm("5").Encode()))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf(rr.Body.String()))
	draft := env.openDrafts()
	c.Assert(draft, qt.HasLen, 1)
	c.Assert(draft[0].Values["original_price"], qt.Equals, "129.90")
	c.Assert(draft[0].Values["original_price_currency"], qt.Equals, "EUR")
	c.Assert(draft[0].Values["purchase_date"], qt.Equals, "2024-03-14")

	form := paperlessForm("6")
	form.Add("title", "twice")
	rr = env.raw(token, path, "application/x-www-form-urlencoded", []byte(form.Encode()))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf(rr.Body.String()))

	form = paperlessForm("7")
	form.Set("owner", "1")
	rr = env.raw(token, path, "application/x-www-form-urlencoded", []byte(form.Encode()))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf(rr.Body.String()))
	c.Assert(rr.Body.String(), qt.Contains, "owner")

	form = paperlessForm("8")
	form.Set("source_url", "file:///etc/passwd")
	rr = env.raw(token, path, "application/x-www-form-urlencoded", []byte(form.Encode()))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf(rr.Body.String()))
	c.Assert(rr.Body.String(), qt.Contains, "source_url")

	form = paperlessForm("9")
	form.Del("source_system")
	rr = env.raw(token, path, "application/x-www-form-urlencoded", []byte(form.Encode()))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf(rr.Body.String()))
	c.Assert(rr.Body.String(), qt.Contains, "source_system")
	c.Assert(env.openDrafts(), qt.HasLen, 1)
}

func TestIntakeDelivery_MultipartWithFile(t *testing.T) {
	c := qt.New(t)
	// A store of its own, so the test can count the blobs in it.
	env, token := newIntakeDeliveryEnv(c, func(p *apiserver.Params) {
		dir := filepath.ToSlash(c.TempDir())
		if !strings.HasPrefix(dir, "/") {
			dir = "/" + dir
		}
		p.UploadLocation = "file://" + dir + "?create_dir=1"
	})
	path := env.groupPath(env.group, "/intake")
	regs := env.groupRegistries()
	ctx := env.groupCtx()
	bucket := must.Must(blob.OpenBucket(context.Background(), env.params.UploadLocation))
	defer bucket.Close()
	// The fixture's group has files of its own, without blobs.
	files := len(must.Must(regs.FileRegistry.List(ctx))) + 1
	const blobs = 1

	contentType, body := multipartDelivery(c, paperlessForm("117"), "file", "invoice-117.pdf", pdf)
	rr := env.raw(token, path, contentType, body)
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf(rr.Body.String()))
	doc := decodeIntakeDocument(c, rr)
	c.Assert(doc.Attributes.FileID, qt.IsNotNil)

	file := must.Must(regs.FileRegistry.Get(ctx, *doc.Attributes.FileID))
	c.Assert(file.File.MIMEType, qt.Equals, "application/pdf")
	// Named as an upload of the file would be.
	c.Assert(file.Title, qt.Matches, `invoice-117-\d+`)
	attached := must.Must(regs.FileRegistry.ListByLinkedEntity(ctx, models.FileLinkDraft, *doc.Attributes.DraftID))
	c.Assert(attached, qt.HasLen, 1)
	c.Assert(must.Must(bucket.Exists(context.Background(), file.File.OriginalPath)), qt.IsTrue)

	// A replay stores the file again only to find it is not needed, and
	// removes the blob.
	contentType, body = multipartDelivery(c, paperlessForm("117"), "file", "invoice-117.pdf", pdf)
	rr = env.raw(token, path, contentType, body)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	c.Assert(must.Must(regs.FileRegistry.List(ctx)), qt.HasLen, files)
	c.Assert(blobCount(c, bucket), qt.Equals, blobs)

	// Another file under the same document is a change; its blob goes too.
	contentType, body = multipartDelivery(c, paperlessForm("117"), "file", "invoice-117.pdf", append(bytes.Clone(pdf), '\n'))
	rr = env.raw(token, path, contentType, body)
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf(rr.Body.String()))
	c.Assert(blobCount(c, bucket), qt.Equals, blobs)

	// A file in another part is refused and not kept.
	contentType, body = multipartDelivery(c, paperlessForm("118"), "document", "invoice-118.pdf", pdf)
	rr = env.raw(token, path, contentType, body)
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf(rr.Body.String()))
	c.Assert(blobCount(c, bucket), qt.Equals, blobs)

	// A malformed field after the file leaves no blob behind.
	fields := paperlessForm("119")
	fields.Set("amount", "lots")
	contentType, body = multipartDelivery(c, fields, "file", "invoice-119.pdf", pdf)
	rr = env.raw(token, path, contentType, body)
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf(rr.Body.String()))
	c.Assert(blobCount(c, bucket), qt.Equals, blobs)
	c.Assert(env.openDrafts(), qt.HasLen, 1)
}

func blobCount(c *qt.C, bucket *blob.Bucket) int {
	c.Helper()
	n := 0
	iter := bucket.List(nil)
	for {
		obj, err := iter.Next(context.Background())
		if err != nil {
			break
		}
		if !obj.IsDir && !strings.Contains(obj.Key, "thumbnails") {
			n++
		}
	}
	return n
}

func TestIntakeDelivery_Limits(t *testing.T) {
	c := qt.New(t)
	env, token := newIntakeDeliveryEnv(c, func(p *apiserver.Params) { p.MaxUploadBytes = 64 })
	path := env.groupPath(env.group, "/intake")

	contentType, body := multipartDelivery(c, paperlessForm("1"), "file", "big.pdf", append(bytes.Clone(pdf), make([]byte, 128)...))
	rr := env.raw(token, path, contentType, body)
	c.Assert(rr.Code, qt.Equals, http.StatusRequestEntityTooLarge, qt.Commentf(rr.Body.String()))

	form := paperlessForm("2")
	form.Set("seller", strings.Repeat("x", 70<<10))
	rr = env.raw(token, path, "application/x-www-form-urlencoded", []byte(form.Encode()))
	c.Assert(rr.Code, qt.Equals, http.StatusRequestEntityTooLarge, qt.Commentf(rr.Body.String()))

	contentType, body = multipartDelivery(c, form, "file", "", nil)
	rr = env.raw(token, path, contentType, body)
	c.Assert(rr.Code, qt.Equals, http.StatusRequestEntityTooLarge, qt.Commentf(rr.Body.String()))

	rr = env.raw(token, path, "text/plain", []byte("title=x"))
	c.Assert(rr.Code, qt.Equals, http.StatusUnsupportedMediaType, qt.Commentf(rr.Body.String()))
	rr = env.raw(token, path, "", []byte("title=x"))
	c.Assert(rr.Code, qt.Equals, http.StatusUnsupportedMediaType, qt.Commentf(rr.Body.String()))
	rr = env.raw(token, path, "application/json", []byte(`["not", "an", "object"]`))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf(rr.Body.String()))
	rr = env.raw(token, path, "application/json", []byte(`{"title": {"nested": true}}`))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf(rr.Body.String()))
	c.Assert(env.openDrafts(), qt.HasLen, 0)
}

func TestIntakeDelivery_JSONBody(t *testing.T) {
	c := qt.New(t)
	env, token := newIntakeDeliveryEnv(c)
	path := env.groupPath(env.group, "/intake")
	const member = `"title":"Invoice","source_system":"paperless-ngx","document_id":"117"`

	for _, tc := range []struct {
		name  string
		body  string
		field string
	}{
		{name: "a member given twice", body: `{` + member + `,"document_id":"118"}`, field: "document_id"},
		{name: "a second object", body: `{` + member + `}{"title":"Other"}`},
		{name: "text after the object", body: `{` + member + `} trailing`},
		{name: "an unclosed object", body: `{` + member},
		{name: "an empty body", body: ``},
	} {
		c.Run(tc.name, func(c *qt.C) {
			rr := env.raw(token, path, "application/json", []byte(tc.body))
			c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf(rr.Body.String()))
			if tc.field != "" {
				c.Check(rr.Body.String(), qt.Contains, tc.field)
				c.Check(rr.Body.String(), qt.Contains, "validation_intake_repeated_field")
			}
		})
	}
	c.Assert(env.openDrafts(), qt.HasLen, 0)

	// A body past the size limit is refused as too large, even when what
	// follows the object is only white space.
	rr := env.raw(token, path, "application/json", []byte(`{`+member+`}`+strings.Repeat(" ", 70<<10)))
	c.Assert(rr.Code, qt.Equals, http.StatusRequestEntityTooLarge, qt.Commentf(rr.Body.String()))
	c.Assert(env.openDrafts(), qt.HasLen, 0)

	// White space after the object is fine.
	rr = env.raw(token, path, "application/json", []byte("{"+member+"}\n\t "))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf(rr.Body.String()))
	c.Assert(env.openDrafts(), qt.HasLen, 1)
}

func TestIntakeDelivery_WhoMayDeliver(t *testing.T) {
	c := qt.New(t)
	env, token := newIntakeDeliveryEnv(c)
	path := env.groupPath(env.group, "/intake")
	form := []byte(paperlessForm("1").Encode())
	const formType = "application/x-www-form-urlencoded"

	// A browser session is refused: the endpoint is for other systems.
	rr := env.session(http.MethodPost, path, map[string]any{"title": "x", "source_system": "s", "document_id": "1"})
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf(rr.Body.String()))
	c.Assert(errorCode(c, rr), qt.Equals, apiserver.IntakeAPITokenRequiredCode)

	rr = env.raw("", path, formType, form)
	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized, qt.Commentf(rr.Body.String()))

	readOnly := env.mint(map[string]any{"scope": "read_only"})
	rr = env.raw(readOnly.Secret, path, formType, form)
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf(rr.Body.String()))
	c.Assert(errorCode(c, rr), qt.Equals, "api_token.read_only")

	other := env.addGroup(models.GroupRoleAdmin)
	rr = env.raw(token, env.groupPath(other, "/intake"), formType, form)
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf(rr.Body.String()))
	c.Assert(errorCode(c, rr), qt.Equals, "api_token.group_not_allowed")

	env.setRole(env.group, models.GroupRoleViewer)
	rr = env.raw(token, path, formType, form)
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf(rr.Body.String()))
	c.Assert(env.openDrafts(), qt.HasLen, 0)

	env.setRole(env.group, models.GroupRoleUser)
	rr = env.raw(token, path, formType, form)
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf(rr.Body.String()))

	// The same document in another group is a document of that group.
	unrestricted := env.mint(map[string]any{"scope": "read_write"})
	rr = env.raw(unrestricted.Secret, env.groupPath(other, "/intake"), formType, form)
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf(rr.Body.String()))
}

// TestIntakeDelivery_SessionOfALostRecord: the group lost the record of the
// first delivery, but its session still carries the document. A delivery of
// the document is refused with its own code and no document to name, with
// the first values too, and makes nothing.
func TestIntakeDelivery_SessionOfALostRecord(t *testing.T) {
	c := qt.New(t)
	env, token := newIntakeDeliveryEnv(c)
	path := env.groupPath(env.group, "/intake")
	form := paperlessForm("117")

	rr := env.raw(token, path, "application/x-www-form-urlencoded", []byte(form.Encode()))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf(rr.Body.String()))
	first := decodeIntakeDocument(c, rr)
	c.Assert(env.groupRegistries().IntakeDocumentRegistry.Delete(env.groupCtx(), first.ID), qt.IsNil)

	changed := paperlessForm("117")
	changed.Set("title", "Another invoice")
	for _, body := range []url.Values{changed, form} {
		rr = env.raw(token, path, "application/x-www-form-urlencoded", []byte(body.Encode()))
		c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf(rr.Body.String()))
		c.Assert(errorCode(c, rr), qt.Equals, apiserver.IntakeDocumentConflictCode)
		c.Assert(rr.Body.String(), qt.Not(qt.Contains), "draft_id")
		c.Assert(env.openDrafts(), qt.HasLen, 1)
	}
}
