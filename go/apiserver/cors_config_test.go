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

	parsed := apiserver.ParseAllowedOrigins(" https://inventario.example.com , https://inventario.example.com,https://app.example.com ,, ")
	c.Assert(parsed, qt.DeepEquals, []string{"https://inventario.example.com", "https://app.example.com"})
}

func TestParseAllowedOrigins_EmptyFallsBackToDevDefaults(t *testing.T) {
	c := qt.New(t)

	parsed := apiserver.ParseAllowedOrigins("")
	c.Assert(parsed, qt.DeepEquals, []string{"http://localhost:5173", "http://localhost:3000"})
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
	c.Assert(allowedRes.Header().Get("Access-Control-Allow-Credentials"), qt.Equals, "true")

	blockedReq := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
	blockedReq.Header.Set("Origin", "https://evil.example.com")
	blockedRes := httptest.NewRecorder()
	handler.ServeHTTP(blockedRes, blockedReq)
	c.Assert(blockedRes.Header().Get("Access-Control-Allow-Origin"), qt.Equals, "")
}
