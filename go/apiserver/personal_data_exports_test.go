package apiserver_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/backup/export"
	"go.5x5.cz/inventario/models"
)

const personalDataExportsPath = "/api/v1/users/me/data-exports"

// newPersonalDataExportEnv is the token test environment with the personal
// data export service the server and the test share, so the test can build
// an export the way the worker does.
func newPersonalDataExportEnv(c *qt.C) (*apiTokenEnv, *export.PersonalDataExportService) {
	c.Helper()
	var svc *export.PersonalDataExportService
	env := newAPITokenEnv(c, func(p *apiserver.Params) {
		svc = export.NewPersonalDataExportService(p.FactorySet, p.UploadLocation)
		p.PersonalDataExports = svc
	})
	return env, svc
}

func requestPersonalDataExport(c *qt.C, env *apiTokenEnv) apiserver.PersonalDataExportView {
	c.Helper()
	rr := env.session(http.MethodPost, personalDataExportsPath, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf(rr.Body.String()))
	var view apiserver.PersonalDataExportView
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &view), qt.IsNil)
	return view
}

func downloadURL(c *qt.C, env *apiTokenEnv, id string) apiserver.PersonalDataExportDownloadURLResponse {
	c.Helper()
	rr := env.session(http.MethodPost, personalDataExportsPath+"/"+id+"/download-url", nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	var resp apiserver.PersonalDataExportDownloadURLResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &resp), qt.IsNil)
	return resp
}

func TestPersonalDataExports_RequestListAndDownload(t *testing.T) {
	c := qt.New(t)
	env, svc := newPersonalDataExportEnv(c)

	view := requestPersonalDataExport(c, env)
	c.Assert(view.Status, qt.Equals, models.PersonalDataExportStatusPending)
	requests := env.auditRows("personal_data_export_request")
	c.Assert(requests, qt.HasLen, 1)
	c.Assert(requests[0].Success, qt.IsTrue)
	c.Assert(*requests[0].EntityID, qt.Equals, view.ID)

	// Not finished yet: listed, not downloadable.
	pending := env.session(http.MethodPost, personalDataExportsPath+"/"+view.ID+"/download-url", nil)
	c.Assert(pending.Code, qt.Equals, http.StatusConflict)
	c.Assert(errorCode(c, pending), qt.Equals, "data_export.not_ready")

	c.Assert(svc.Process(context.Background(), view.ID), qt.IsNil)

	rr := env.session(http.MethodGet, personalDataExportsPath, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.String(), qt.Not(qt.Contains), "blob_key")
	var list apiserver.PersonalDataExportsListResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &list), qt.IsNil)
	c.Assert(list.Exports, qt.HasLen, 1)
	c.Assert(list.Exports[0].Status, qt.Equals, models.PersonalDataExportStatusCompleted)
	c.Assert(list.Exports[0].SizeBytes > 0, qt.IsTrue)
	c.Assert(list.RetentionDays, qt.Equals, 7)
	c.Assert(list.NextRequestAt, qt.IsNotNil)

	signed := downloadURL(c, env, view.ID)
	download := env.do(apiTokenRequest{method: http.MethodGet, path: signed.URL})
	c.Assert(download.Code, qt.Equals, http.StatusOK, qt.Commentf(download.Body.String()))
	c.Assert(download.Header().Get("Content-Type"), qt.Equals, "application/gzip")
	c.Assert(download.Header().Get("Content-Disposition"), qt.Contains, "attachment")
	gz, err := gzip.NewReader(download.Body)
	c.Assert(err, qt.IsNil)
	first, err := tar.NewReader(gz).Next()
	c.Assert(err, qt.IsNil)
	c.Assert(first.Name, qt.Equals, "manifest.json")

	downloads := env.auditRows("personal_data_export_download")
	c.Assert(downloads, qt.HasLen, 1)
	c.Assert(downloads[0].Success, qt.IsTrue)
	c.Assert(*downloads[0].UserID, qt.Equals, env.user.ID)
}

func TestPersonalDataExports_OncePerDay(t *testing.T) {
	c := qt.New(t)
	env, _ := newPersonalDataExportEnv(c)

	requestPersonalDataExport(c, env)
	rr := env.session(http.MethodPost, personalDataExportsPath, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusTooManyRequests, qt.Commentf(rr.Body.String()))
	c.Assert(errorCode(c, rr), qt.Equals, "data_export.rate_limited")
	retryAfter, err := strconv.Atoi(rr.Header().Get("Retry-After"))
	c.Assert(err, qt.IsNil)
	c.Assert(retryAfter > 23*3600 && retryAfter <= 24*3600, qt.IsTrue, qt.Commentf("Retry-After %d", retryAfter))

	var body struct {
		Errors []struct {
			Meta map[string]any `json:"meta"`
		} `json:"errors"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
	retryAt, err := time.Parse(time.RFC3339, body.Errors[0].Meta["retry_at"].(string))
	c.Assert(err, qt.IsNil)
	c.Assert(retryAt.After(time.Now().Add(23*time.Hour)), qt.IsTrue)

	rows := env.auditRows("personal_data_export_request")
	c.Assert(rows, qt.HasLen, 2)
	failed := 0
	for _, row := range rows {
		if !row.Success {
			failed++
		}
	}
	c.Assert(failed, qt.Equals, 1)
}

func TestPersonalDataExports_OnlyTheOwnerReachesThem(t *testing.T) {
	c := qt.New(t)
	env, svc := newPersonalDataExportEnv(c)
	view := requestPersonalDataExport(c, env)
	c.Assert(svc.Process(context.Background(), view.ID), qt.IsNil)
	signed := downloadURL(c, env, view.ID)

	other := must.Must(env.params.FactorySet.UserRegistry.Create(context.Background(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: env.user.TenantID},
		Email:               "other@example.com",
		Name:                "Other",
		IsActive:            true,
	}))
	otherHeaders := map[string][]string{
		"Authorization": {"Bearer " + createTestJWTToken(other.ID)},
		"X-CSRF-Token":  {must.Must(env.csrf.GenerateToken(context.Background(), other.ID))},
	}

	rr := env.do(apiTokenRequest{method: http.MethodPost, path: personalDataExportsPath + "/" + view.ID + "/download-url", headers: otherHeaders})
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
	c.Assert(errorCode(c, rr), qt.Equals, "data_export.not_found")

	list := env.do(apiTokenRequest{method: http.MethodGet, path: personalDataExportsPath, headers: otherHeaders})
	c.Assert(list.Code, qt.Equals, http.StatusOK)
	var resp apiserver.PersonalDataExportsListResponse
	c.Assert(json.Unmarshal(list.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.Exports, qt.HasLen, 0)

	// A signed URL with the uid swapped for another user's fails the
	// signature.
	u := must.Must(url.Parse(signed.URL))
	q := u.Query()
	q.Set("uid", other.ID)
	u.RawQuery = q.Encode()
	tampered := env.do(apiTokenRequest{method: http.MethodGet, path: u.String()})
	c.Assert(tampered.Code, qt.Equals, http.StatusUnauthorized)

	// The URL is bound to the session it was minted for.
	otherSession := env.do(apiTokenRequest{method: http.MethodGet, path: signed.URL, cookies: []*http.Cookie{refreshCookie("another-session")}})
	c.Assert(otherSession.Code, qt.Equals, http.StatusUnauthorized)
}

func TestPersonalDataExports_RefusedWhileImpersonating(t *testing.T) {
	c := qt.New(t)
	env, svc := newPersonalDataExportEnv(c)
	existing := requestPersonalDataExport(c, env)
	c.Assert(svc.Process(context.Background(), existing.ID), qt.IsNil)

	_, operatorToken := WithBackofficeAdmin(t, env.params)
	rr := doImpersonateStart(c, env.handler, operatorToken, env.user.ID, map[string]any{"reason": "support ticket"})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	var login apiserver.LoginResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &login), qt.IsNil)
	headers := map[string][]string{
		"Authorization": {"Bearer " + login.AccessToken},
		"X-CSRF-Token":  {login.CSRFToken},
	}

	create := env.do(apiTokenRequest{method: http.MethodPost, path: personalDataExportsPath, headers: headers})
	c.Assert(create.Code, qt.Equals, http.StatusForbidden, qt.Commentf(create.Body.String()))
	c.Assert(errorCode(c, create), qt.Equals, "data_export.impersonation_forbidden")

	mint := env.do(apiTokenRequest{method: http.MethodPost, path: personalDataExportsPath + "/" + existing.ID + "/download-url", headers: headers})
	c.Assert(mint.Code, qt.Equals, http.StatusForbidden)
	c.Assert(errorCode(c, mint), qt.Equals, "data_export.impersonation_forbidden")

	// Listing is allowed: support can see that an export exists.
	list := env.do(apiTokenRequest{method: http.MethodGet, path: personalDataExportsPath, headers: headers})
	c.Assert(list.Code, qt.Equals, http.StatusOK)

	// A URL minted for the user's own session does not work from the
	// impersonation session's browser either.
	signed := downloadURL(c, env, existing.ID)
	download := env.do(apiTokenRequest{method: http.MethodGet, path: signed.URL, cookies: []*http.Cookie{refreshCookie("imp:some-jti")}})
	c.Assert(download.Code, qt.Not(qt.Equals), http.StatusOK)

	rows := env.auditRows("personal_data_export_download")
	for _, row := range rows {
		c.Assert(row.Success, qt.IsFalse)
	}
}

func TestPersonalDataExports_RestoreUploadRefusesThem(t *testing.T) {
	c := qt.New(t)
	env, svc := newPersonalDataExportEnv(c)
	view := requestPersonalDataExport(c, env)
	c.Assert(svc.Process(context.Background(), view.ID), qt.IsNil)
	_, reader, err := svc.Open(context.Background(), env.user.ID, view.ID)
	c.Assert(err, qt.IsNil)
	defer reader.Close()

	rr := uploadRestoreFile(c, env, "inventario-personal-data.tar.gz", reader)
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf(rr.Body.String()))
	c.Assert(errorCode(c, rr), qt.Equals, "restore.personal_data_export")
}

// uploadRestoreFile uploads content as a backup to restore into the user's
// group, as the backup page does.
func uploadRestoreFile(c *qt.C, env *apiTokenEnv, name string, content io.Reader) *httptest.ResponseRecorder {
	c.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreatePart(CreateFormFileMIME("files", name, "application/gzip"))
	c.Assert(err, qt.IsNil)
	_, err = io.Copy(part, content)
	c.Assert(err, qt.IsNil)
	c.Assert(w.Close(), qt.IsNil)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/g/"+env.group.Slug+"/uploads/restores", &body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	r.Header.Set("Authorization", "Bearer "+createTestJWTToken(env.user.ID))
	r.Header.Set("X-CSRF-Token", must.Must(env.csrf.GenerateToken(context.Background(), env.user.ID)))
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, r)
	return rr
}

// refreshCookie is the refresh token cookie a browser session sends.
func refreshCookie(value string) *http.Cookie {
	return &http.Cookie{Name: appctx.RefreshTokenCookieName, Value: value, Path: "/api/v1", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode}
}
