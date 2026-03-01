package apiserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
)

func TestParseAllowedOrigins(t *testing.T) {
	c := qt.New(t)

	parsed, err := apiserver.ParseAllowedOrigins(" https://inventario.example.com , https://inventario.example.com,https://app.example.com ,, ")
	c.Assert(err, qt.IsNil)
	c.Assert(parsed, qt.DeepEquals, []string{"https://inventario.example.com", "https://app.example.com"})
}

func TestParseAllowedOrigins_EmptyIsFailClosed(t *testing.T) {
	c := qt.New(t)

	parsed, err := apiserver.ParseAllowedOrigins("")
	c.Assert(err, qt.IsNil)
	c.Assert(parsed, qt.HasLen, 0)
}

func TestParseAllowedOrigins_RejectsUnsafeValues(t *testing.T) {
	c := qt.New(t)

	_, err := apiserver.ParseAllowedOrigins("https://inventario.example.com,*")
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "unsafe CORS origin")

	_, err = apiserver.ParseAllowedOrigins("null")
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "unsafe CORS origin")
}

func TestNewCORSMiddleware_OnlyAllowsConfiguredOrigins(t *testing.T) {
	c := qt.New(t)

	cfg := apiserver.DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"https://inventario.example.com"}

	handler := apiserver.NewCORSMiddleware(cfg).Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	allowedReq := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
	allowedReq.Header.Set("Origin", "https://inventario.example.com")
	allowedRes := httptest.NewRecorder()
	handler.ServeHTTP(allowedRes, allowedReq)
	c.Assert(allowedRes.Header().Get("Access-Control-Allow-Origin"), qt.Equals, "https://inventario.example.com")

	blockedReq := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
	blockedReq.Header.Set("Origin", "https://evil.example.com")
	blockedRes := httptest.NewRecorder()
	handler.ServeHTTP(blockedRes, blockedReq)
	c.Assert(blockedRes.Header().Get("Access-Control-Allow-Origin"), qt.Equals, "")
}

func TestNewCORSMiddleware_DefaultsAllowCredentialsToTrue(t *testing.T) {
	c := qt.New(t)

	cfg := apiserver.CORSConfig{
		AllowedOrigins: []string{"https://inventario.example.com"},
	}

	handler := apiserver.NewCORSMiddleware(cfg).Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
	req.Header.Set("Origin", "https://inventario.example.com")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	c.Assert(res.Header().Get("Access-Control-Allow-Credentials"), qt.Equals, "true")
}
