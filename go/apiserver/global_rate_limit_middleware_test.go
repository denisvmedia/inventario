package apiserver_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
