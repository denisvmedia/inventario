package apiserver_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/services"
)

type errGlobalLimiter struct{}

func (errGlobalLimiter) Check(context.Context, string) (services.RateLimitResult, error) {
	return services.RateLimitResult{}, errors.New("backend unavailable")
}

func (errGlobalLimiter) RateLimitHits() uint64 {
	return 0
}

func TestGlobalRateLimitMiddleware_BlocksAndSetsHeaders(t *testing.T) {
	c := qt.New(t)

	limiter := services.NewInMemoryGlobalRateLimiter(2, time.Hour)
	t.Cleanup(limiter.Stop)
	handler := apiserver.GlobalRateLimitMiddleware(limiter, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	makeRequest := func(ip string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
		req.RemoteAddr = ip + ":1234"
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}

	first := makeRequest("10.0.0.1")
	c.Assert(first.Code, qt.Equals, http.StatusOK)
	c.Assert(first.Header().Get("X-RateLimit-Limit"), qt.Equals, "2")
	c.Assert(first.Header().Get("X-RateLimit-Remaining"), qt.Equals, "1")
	c.Assert(first.Header().Get("X-RateLimit-Reset"), qt.Not(qt.Equals), "")

	second := makeRequest("10.0.0.1")
	c.Assert(second.Code, qt.Equals, http.StatusOK)
	c.Assert(second.Header().Get("X-RateLimit-Remaining"), qt.Equals, "0")

	third := makeRequest("10.0.0.1")
	c.Assert(third.Code, qt.Equals, http.StatusTooManyRequests)
	c.Assert(third.Header().Get("X-RateLimit-Limit"), qt.Equals, "2")
	c.Assert(third.Header().Get("X-RateLimit-Remaining"), qt.Equals, "0")
	c.Assert(third.Header().Get("Retry-After"), qt.Not(qt.Equals), "")
	c.Assert(limiter.RateLimitHits(), qt.Equals, uint64(1))
}

func TestGlobalRateLimitMiddleware_FailsOpenOnLimiterErrors(t *testing.T) {
	c := qt.New(t)

	handler := apiserver.GlobalRateLimitMiddleware(errGlobalLimiter{}, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
	req.RemoteAddr = "10.0.0.2:4321"
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	c.Assert(res.Code, qt.Equals, http.StatusOK)
}

func TestGlobalRateLimitMiddleware_UsesXForwardedForOnlyForTrustedProxies(t *testing.T) {
	c := qt.New(t)

	trustedNets, err := apiserver.ParseTrustedProxyCIDRs("10.0.0.0/8")
	c.Assert(err, qt.IsNil)

	t.Run("trusted proxy honors X-Forwarded-For", func(t *testing.T) {
		c := qt.New(t)
		limiter := services.NewInMemoryGlobalRateLimiter(1, time.Hour)
		t.Cleanup(limiter.Stop)
		handler := apiserver.GlobalRateLimitMiddleware(limiter, trustedNets)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req1 := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
		req1.RemoteAddr = "10.1.1.1:1234"
		req1.Header.Set("X-Forwarded-For", "203.0.113.10")
		res1 := httptest.NewRecorder()
		handler.ServeHTTP(res1, req1)
		c.Assert(res1.Code, qt.Equals, http.StatusOK)

		req2 := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
		req2.RemoteAddr = "10.1.1.1:1235"
		req2.Header.Set("X-Forwarded-For", "203.0.113.11")
		res2 := httptest.NewRecorder()
		handler.ServeHTTP(res2, req2)
		c.Assert(res2.Code, qt.Equals, http.StatusOK)
	})

	t.Run("untrusted proxy ignores X-Forwarded-For", func(t *testing.T) {
		c := qt.New(t)
		limiter := services.NewInMemoryGlobalRateLimiter(1, time.Hour)
		t.Cleanup(limiter.Stop)
		handler := apiserver.GlobalRateLimitMiddleware(limiter, trustedNets)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req1 := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
		req1.RemoteAddr = "198.51.100.10:1234"
		req1.Header.Set("X-Forwarded-For", "203.0.113.10")
		res1 := httptest.NewRecorder()
		handler.ServeHTTP(res1, req1)
		c.Assert(res1.Code, qt.Equals, http.StatusOK)

		req2 := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
		req2.RemoteAddr = "198.51.100.10:1235"
		req2.Header.Set("X-Forwarded-For", "203.0.113.99")
		res2 := httptest.NewRecorder()
		handler.ServeHTTP(res2, req2)
		c.Assert(res2.Code, qt.Equals, http.StatusTooManyRequests)
	})
}

// TestAPIServer_GlobalRateLimitExemptions verifies the route-tier exemptions introduced
// in issue #1208: the /auth/* subrouter must remain accessible when the global per-IP
// rate-limit budget has been exhausted by requests to other public endpoints.
func TestAPIServer_GlobalRateLimitExemptions(t *testing.T) {
	c := qt.New(t)

	params, _ := newParamsAreaRegistryOnly()

	// Limit of 1 request per hour means a single hit to a globally-limited public
	// endpoint exhausts the entire budget for that IP.
	limiter := services.NewInMemoryGlobalRateLimiter(1, time.Hour)
	t.Cleanup(limiter.Stop)
	params.GlobalRateLimiter = limiter

	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	const clientIP = "203.0.113.42:1234"

	makeReq := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.RemoteAddr = clientIP
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}

	// First request to a globally-rate-limited public endpoint (/register):
	// budget = 1, so this request is allowed (response is some non-429 status).
	first := makeReq(http.MethodPost, "/api/v1/register",
		`{"email":"a@example.com","password":"pass1234","name":"Alice"}`)
	c.Assert(first.Code, qt.Not(qt.Equals), http.StatusTooManyRequests)

	// Second request to the same endpoint: budget exhausted — must return 429.
	second := makeReq(http.MethodPost, "/api/v1/register",
		`{"email":"b@example.com","password":"pass1234","name":"Bob"}`)
	c.Assert(second.Code, qt.Equals, http.StatusTooManyRequests)

	// /auth/login is exempt from the global limiter and must still be reachable.
	// Wrong credentials yield 401; anything other than 429 proves the route is accessible.
	login := makeReq(http.MethodPost, "/api/v1/auth/login",
		`{"email":"nobody@example.com","password":"wrongpassword"}`)
	c.Assert(login.Code, qt.Not(qt.Equals), http.StatusTooManyRequests)
}
