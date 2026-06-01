package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/aivision"
	"github.com/denisvmedia/inventario/internal/aivision/mock"
	"github.com/denisvmedia/inventario/internal/checkers"
	"github.com/denisvmedia/inventario/services"
)

// newPublicScanParams builds an apiserver.Params with the public scan
// endpoint enabled and wired to the given provider. The public route lives
// outside the JWT/group middleware, so no auth header / group slug is
// needed to drive it.
func newPublicScanParams(provider aivision.Provider, cfg services.CommodityScanConfig) apiserver.Params {
	params, _, _ := newParams()
	params.CommodityScanService = services.NewCommodityScanService(provider, params.FactorySet.CommodityScanAuditRegistry, cfg)
	params.PublicScanEnabled = true
	return params
}

// publicScanURL is the unauthenticated scan endpoint path.
const publicScanURL = "/api/v1/public/commodities/scan"

func postPublicScan(c *qt.C, handler http.Handler, photos []struct {
	name string
	mime string
	body []byte
},
) *httptest.ResponseRecorder {
	body, contentType := buildScanMultipart(c, photos)
	req := httptest.NewRequest(http.MethodPost, publicScanURL, body)
	req.Header.Set("Content-Type", contentType)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func onePhoto(body []byte) []struct {
	name string
	mime string
	body []byte
} {
	return []struct {
		name string
		mime string
		body []byte
	}{{name: "a.jpg", mime: "image/jpeg", body: body}}
}

func TestCommodityScanPublic_HappyPath_NoRowsWritten(t *testing.T) {
	c := qt.New(t)

	params := newPublicScanParams(mock.New(), services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20})
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := postPublicScan(c, handler, onePhoto(bytes.Repeat([]byte("a"), 128)))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.type"), "commodity_scan")
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.fields.name.value"), "Sample Wireless Headphones")

	// The anonymous path persists nothing: the audit registry has no
	// entry point that the public handler ever calls. A probe count for
	// the seeded test user stays at zero (no audit row attributed there),
	// and there is no tenant/user the public handler could have written
	// under anyway.
	count, err := params.FactorySet.CommodityScanAuditRegistry.CountRecentForUser(
		context.Background(), "tenant-1", "user-1", time.Now().Add(-1*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestCommodityScanPublic_FlagOff_NotMounted(t *testing.T) {
	c := qt.New(t)

	params := newPublicScanParams(mock.New(), services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20})
	params.PublicScanEnabled = false // explicit: route must not be mounted
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := postPublicScan(c, handler, onePhoto([]byte("aaa")))
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestCommodityScanPublic_NilService_NotMounted(t *testing.T) {
	c := qt.New(t)

	// PublicScanEnabled true but no scan service → the route stays absent
	// (the mount predicate requires a non-nil service), so the FE never
	// reaches an anonymous endpoint that could only 503 anyway.
	params, _, _ := newParams()
	params.PublicScanEnabled = true
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := postPublicScan(c, handler, onePhoto([]byte("aaa")))
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestCommodityScanPublic_ServiceWithNilProvider_503(t *testing.T) {
	c := qt.New(t)

	// A wired service whose provider is nil is the "feature off in this
	// deployment" shape: the route is mounted (service non-nil) but every
	// call surfaces the typed 503 the FE banner already renders.
	params := newPublicScanParams(nil, services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20})
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := postPublicScan(c, handler, onePhoto([]byte("aaa")))
	c.Assert(rr.Code, qt.Equals, http.StatusServiceUnavailable)
	assertErrorCode(t, c, rr.Body.Bytes(), "commodity_scan.provider_disabled")
}

func TestCommodityScanPublic_NoPhotos_422(t *testing.T) {
	c := qt.New(t)

	params := newPublicScanParams(mock.New(), services.CommodityScanConfig{MaxPhotos: 5})
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// No photo parts at all.
	rr := postPublicScan(c, handler, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), "commodity_scan.no_photos")
}

func TestCommodityScanPublic_UnsupportedMIME_415(t *testing.T) {
	c := qt.New(t)

	params := newPublicScanParams(mock.New(), services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20})
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := postPublicScan(c, handler, []struct {
		name string
		mime string
		body []byte
	}{{name: "a.gif", mime: "image/gif", body: []byte("aaa")}})
	c.Assert(rr.Code, qt.Equals, http.StatusUnsupportedMediaType)
	assertErrorCode(t, c, rr.Body.Bytes(), "commodity_scan.unsupported_mime")
}

func TestCommodityScanPublic_PhotoTooLarge_413(t *testing.T) {
	c := qt.New(t)

	params := newPublicScanParams(mock.New(), services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 32})
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := postPublicScan(c, handler, onePhoto(bytes.Repeat([]byte("a"), 1024)))
	c.Assert(rr.Code, qt.Equals, http.StatusRequestEntityTooLarge)
	assertErrorCode(t, c, rr.Body.Bytes(), "commodity_scan.photo_too_large")
}

func TestCommodityScanPublic_ProviderTimeout_504(t *testing.T) {
	c := qt.New(t)

	provider := mock.New(mock.WithDefaultError(aivision.ErrProviderTimeout))
	params := newPublicScanParams(provider, services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20})
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := postPublicScan(c, handler, onePhoto([]byte("aaa")))
	c.Assert(rr.Code, qt.Equals, http.StatusGatewayTimeout)
	assertErrorCode(t, c, rr.Body.Bytes(), "commodity_scan.provider_timeout")
}

// stubScanLimiter is a minimal AuthRateLimiter that returns canned public-
// scan decisions and allows everything else. It lets the 429 tests force a
// per-IP or global-cap rejection without driving the real sliding window.
type stubScanLimiter struct {
	services.AuthRateLimiter // embed the no-op for the methods we don't care about
	ipAllowed                bool
	globalAllowed            bool
}

func newStubScanLimiter(ipAllowed, globalAllowed bool) *stubScanLimiter {
	return &stubScanLimiter{
		AuthRateLimiter: services.NewNoOpAuthRateLimiter(),
		ipAllowed:       ipAllowed,
		globalAllowed:   globalAllowed,
	}
}

func (s *stubScanLimiter) CheckPublicScanAttempt(_ context.Context, _ string) (services.RateLimitResult, error) {
	return services.RateLimitResult{Allowed: s.ipAllowed, Limit: 3, ResetAt: time.Now().Add(time.Hour)}, nil
}

func (s *stubScanLimiter) CheckPublicScanGlobalCap(_ context.Context) (services.RateLimitResult, error) {
	return services.RateLimitResult{Allowed: s.globalAllowed, Limit: 200, ResetAt: time.Now().Add(24 * time.Hour)}, nil
}

func TestCommodityScanPublic_PerIPRateLimited_429(t *testing.T) {
	c := qt.New(t)

	params := newPublicScanParams(mock.New(), services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20})
	// Global cap allows, per-IP denies.
	params.AuthRateLimiter = newStubScanLimiter(false, true)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := postPublicScan(c, handler, onePhoto([]byte("aaa")))
	c.Assert(rr.Code, qt.Equals, http.StatusTooManyRequests)
	c.Assert(rr.Header().Get("Retry-After"), qt.Not(qt.Equals), "")
}

func TestCommodityScanPublic_GlobalCapRateLimited_429(t *testing.T) {
	c := qt.New(t)

	params := newPublicScanParams(mock.New(), services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20})
	// Global cap denies (checked first), per-IP would allow.
	params.AuthRateLimiter = newStubScanLimiter(true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := postPublicScan(c, handler, onePhoto([]byte("aaa")))
	c.Assert(rr.Code, qt.Equals, http.StatusTooManyRequests)
	c.Assert(rr.Header().Get("Retry-After"), qt.Not(qt.Equals), "")
}

func TestFeatureFlags_PublicScanSurfaced(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams()
	params.PublicScanEnabled = true
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var flags map[string]any
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &flags), qt.IsNil)
	c.Assert(flags["public_scan"], qt.Equals, true)
}

func TestFeatureFlags_PublicScanFalseByDefault(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams() // PublicScanEnabled defaults to false
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var flags map[string]any
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &flags), qt.IsNil)
	c.Assert(flags["public_scan"], qt.Equals, false)
}
