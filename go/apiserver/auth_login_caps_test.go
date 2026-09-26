package apiserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/apiserver"
)

// #2479 / #967 H8: the login body is length-capped. Neither cap is a security
// boundary — the request body is capped upstream and bcrypt ignores input past
// 72 bytes — so what they buy is that an absurd value is refused at the edge
// instead of travelling through the rate limiter, a registry lookup and a hash
// comparison to arrive at the same 401.

// postLogin sends a login body and returns the recorder.
func postLogin(t *testing.T, handler http.Handler, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestLogin_FieldLengthCaps(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)

	c.Run("an over-long email is refused", func(c *qt.C) {
		rr := postLogin(t, env.handler, map[string]any{
			"email":    strings.Repeat("a", 244) + "@example.com", // 256 bytes
			"password": "Password123",
		})
		c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)
		c.Assert(rr.Body.String(), qt.Contains, "too long")
	})

	c.Run("an over-long password is refused", func(c *qt.C) {
		rr := postLogin(t, env.handler, map[string]any{
			"email":    "someone@example.com",
			"password": strings.Repeat("x", 1025),
		})
		c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)
		c.Assert(rr.Body.String(), qt.Contains, "too long")
	})

	// The boundary matters more than the middle: a 254-byte address is legal per
	// RFC 5321 and has to still reach the lookup, where it fails as an unknown
	// account rather than as a malformed request.
	c.Run("an email at the cap reaches the lookup", func(c *qt.C) {
		rr := postLogin(t, env.handler, map[string]any{
			"email":    strings.Repeat("a", 242) + "@example.com", // 254 bytes
			"password": "Password123",
		})
		c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized,
			qt.Commentf("a legal 254-byte address was rejected as malformed: %s", rr.Body.String()))
	})

	c.Run("a password at the cap reaches the lookup", func(c *qt.C) {
		rr := postLogin(t, env.handler, map[string]any{
			"email":    "someone@example.com",
			"password": strings.Repeat("x", 1024),
		})
		c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
	})

	// The caps sit after the empty check, so the message for an absent field is
	// unchanged.
	c.Run("an empty field still says required", func(c *qt.C) {
		rr := postLogin(t, env.handler, map[string]any{"email": "", "password": ""})
		c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)
		c.Assert(rr.Body.String(), qt.Contains, "required")
	})
}

// The caps are exported nowhere, so this pins the numbers against the reasons
// given for them: RFC 5321's 254-byte address, and a generous password bound
// that is well past bcrypt's own 72-byte limit.
func TestLogin_CapsMatchTheirStatedReasons(t *testing.T) {
	c := qt.New(t)
	c.Assert(apiserver.LoginEmailMaxLenForTest, qt.Equals, 254)
	c.Assert(apiserver.LoginPasswordMaxLenForTest > 72, qt.IsTrue,
		qt.Commentf("the password cap must not be tighter than what bcrypt already ignores"))
}
