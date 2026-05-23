package apiserver_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/aivision"
	"github.com/denisvmedia/inventario/internal/aivision/mock"
	"github.com/denisvmedia/inventario/internal/checkers"
	"github.com/denisvmedia/inventario/services"
)

// buildScanMultipart returns the body + content-type for a scan request
// carrying the given (filename, mime, payload) photos.
func buildScanMultipart(c *qt.C, photos []struct {
	name string
	mime string
	body []byte
}) (*bytes.Buffer, string) {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	for _, p := range photos {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", `form-data; name="photos"; filename="`+p.name+`"`)
		h.Set("Content-Type", p.mime)
		part, err := w.CreatePart(h)
		c.Assert(err, qt.IsNil)
		_, err = io.Copy(part, bytes.NewReader(p.body))
		c.Assert(err, qt.IsNil)
	}
	contentType := w.FormDataContentType()
	c.Assert(w.Close(), qt.IsNil)
	return buf, contentType
}

func newScanParams(provider aivision.Provider, cfg services.CommodityScanConfig) (params apiserver.Params, userID, groupSlug string) {
	params, testUser, testGroup := newParams()
	params.CommodityScanService = services.NewCommodityScanService(provider, params.FactorySet.CommodityScanAuditRegistry, cfg)
	return params, testUser.ID, testGroup.Slug
}

func TestCommodityScan_HappyPath(t *testing.T) {
	c := qt.New(t)

	params, userID, slug := newScanParams(mock.New(), services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20, RateLimitPerHour: 100})
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	body, contentType := buildScanMultipart(c, []struct {
		name string
		mime string
		body []byte
	}{
		{name: "front.jpg", mime: "image/jpeg", body: bytes.Repeat([]byte("a"), 128)},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/g/"+slug+"/commodities/scan", body)
	req.Header.Set("Content-Type", contentType)
	addTestUserAuthHeader(req, userID)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var resp struct {
		Data struct {
			Type       string         `json:"type"`
			Attributes map[string]any `json:"attributes"`
		} `json:"data"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.Data.Type, qt.Equals, "commodity_scan")
	c.Assert(resp.Data.Attributes["fields"], qt.IsNotNil)
}

func TestCommodityScan_MissingMultipart(t *testing.T) {
	c := qt.New(t)

	params, userID, slug := newScanParams(mock.New(), services.CommodityScanConfig{MaxPhotos: 5})
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Content-Type that isn't multipart at all: r.MultipartReader()
	// returns an error which the handler maps to 400.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/g/"+slug+"/commodities/scan", bytes.NewReader([]byte("plain body")))
	req.Header.Set("Content-Type", "text/plain")
	addTestUserAuthHeader(req, userID)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)
}

func TestCommodityScan_NoPhotos(t *testing.T) {
	c := qt.New(t)

	params, userID, slug := newScanParams(mock.New(), services.CommodityScanConfig{MaxPhotos: 5})
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	c.Assert(w.WriteField("hint", "a brand"), qt.IsNil)
	c.Assert(w.Close(), qt.IsNil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/g/"+slug+"/commodities/scan", buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	addTestUserAuthHeader(req, userID)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), "commodity_scan.no_photos")
}

func TestCommodityScan_TooManyPhotos(t *testing.T) {
	c := qt.New(t)

	params, userID, slug := newScanParams(mock.New(), services.CommodityScanConfig{MaxPhotos: 1, MaxPhotoBytes: 1 << 20})
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	body, contentType := buildScanMultipart(c, []struct {
		name string
		mime string
		body []byte
	}{
		{"a.jpg", "image/jpeg", []byte("aaa")},
		{"b.jpg", "image/jpeg", []byte("bbb")},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/g/"+slug+"/commodities/scan", body)
	req.Header.Set("Content-Type", contentType)
	addTestUserAuthHeader(req, userID)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), "commodity_scan.too_many_photos")
}

func TestCommodityScan_PhotoTooLarge(t *testing.T) {
	c := qt.New(t)

	params, userID, slug := newScanParams(mock.New(), services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 32})
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	body, contentType := buildScanMultipart(c, []struct {
		name string
		mime string
		body []byte
	}{
		{"a.jpg", "image/jpeg", bytes.Repeat([]byte("a"), 1024)},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/g/"+slug+"/commodities/scan", body)
	req.Header.Set("Content-Type", contentType)
	addTestUserAuthHeader(req, userID)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusRequestEntityTooLarge)
	assertErrorCode(t, c, rr.Body.Bytes(), "commodity_scan.photo_too_large")
}

func TestCommodityScan_UnsupportedMIME(t *testing.T) {
	c := qt.New(t)

	params, userID, slug := newScanParams(mock.New(), services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20})
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	body, contentType := buildScanMultipart(c, []struct {
		name string
		mime string
		body []byte
	}{
		{"a.gif", "image/gif", []byte("aaa")},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/g/"+slug+"/commodities/scan", body)
	req.Header.Set("Content-Type", contentType)
	addTestUserAuthHeader(req, userID)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusUnsupportedMediaType)
	assertErrorCode(t, c, rr.Body.Bytes(), "commodity_scan.unsupported_mime")
}

func TestCommodityScan_ProviderDisabled(t *testing.T) {
	c := qt.New(t)

	params, userID, slug := newScanParams(nil, services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20})
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	body, contentType := buildScanMultipart(c, []struct {
		name string
		mime string
		body []byte
	}{
		{"a.jpg", "image/jpeg", []byte("aaa")},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/g/"+slug+"/commodities/scan", body)
	req.Header.Set("Content-Type", contentType)
	addTestUserAuthHeader(req, userID)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusServiceUnavailable)
	assertErrorCode(t, c, rr.Body.Bytes(), "commodity_scan.provider_disabled")
}

func TestCommodityScan_ProviderTimeout(t *testing.T) {
	c := qt.New(t)

	provider := mock.New(mock.WithDefaultError(aivision.ErrProviderTimeout))
	params, userID, slug := newScanParams(provider, services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20})
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	body, contentType := buildScanMultipart(c, []struct {
		name string
		mime string
		body []byte
	}{
		{"a.jpg", "image/jpeg", []byte("aaa")},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/g/"+slug+"/commodities/scan", body)
	req.Header.Set("Content-Type", contentType)
	addTestUserAuthHeader(req, userID)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusGatewayTimeout)
	assertErrorCode(t, c, rr.Body.Bytes(), "commodity_scan.provider_timeout")
}

func TestCommodityScan_ProviderUnavailable(t *testing.T) {
	c := qt.New(t)

	provider := mock.New(mock.WithDefaultError(aivision.ErrProviderUnavailable))
	params, userID, slug := newScanParams(provider, services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20})
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	body, contentType := buildScanMultipart(c, []struct {
		name string
		mime string
		body []byte
	}{
		{"a.jpg", "image/jpeg", []byte("aaa")},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/g/"+slug+"/commodities/scan", body)
	req.Header.Set("Content-Type", contentType)
	addTestUserAuthHeader(req, userID)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusBadGateway)
	assertErrorCode(t, c, rr.Body.Bytes(), "commodity_scan.provider_error")
}

func TestCommodityScan_ProviderError(t *testing.T) {
	c := qt.New(t)

	provider := mock.New(mock.WithDefaultError(errors.New("unknown")))
	params, userID, slug := newScanParams(provider, services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20})
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	body, contentType := buildScanMultipart(c, []struct {
		name string
		mime string
		body []byte
	}{
		{"a.jpg", "image/jpeg", []byte("aaa")},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/g/"+slug+"/commodities/scan", body)
	req.Header.Set("Content-Type", contentType)
	addTestUserAuthHeader(req, userID)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusBadGateway)
	assertErrorCode(t, c, rr.Body.Bytes(), "commodity_scan.provider_error")
}

func TestCommodityScan_RateLimited(t *testing.T) {
	c := qt.New(t)

	params, userID, slug := newScanParams(mock.New(), services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20, RateLimitPerHour: 1})
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	makeReq := func() *httptest.ResponseRecorder {
		body, contentType := buildScanMultipart(c, []struct {
			name string
			mime string
			body []byte
		}{
			{"a.jpg", "image/jpeg", []byte("aaa")},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/g/"+slug+"/commodities/scan", body)
		req.Header.Set("Content-Type", contentType)
		addTestUserAuthHeader(req, userID)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr
	}

	first := makeReq()
	c.Assert(first.Code, qt.Equals, http.StatusOK)

	second := makeReq()
	c.Assert(second.Code, qt.Equals, http.StatusTooManyRequests)
	assertErrorCode(t, c, second.Body.Bytes(), "commodity_scan.rate_limited")
}

func TestCommodityScan_SuccessResponseFields(t *testing.T) {
	c := qt.New(t)

	params, userID, slug := newScanParams(mock.New(), services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20, RateLimitPerHour: 100})
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	body, contentType := buildScanMultipart(c, []struct {
		name string
		mime string
		body []byte
	}{
		{"a.jpg", "image/jpeg", []byte("aaa")},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/g/"+slug+"/commodities/scan", body)
	req.Header.Set("Content-Type", contentType)
	addTestUserAuthHeader(req, userID)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.type"), "commodity_scan")
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.fields.name.value"), "Sample Wireless Headphones")
}
