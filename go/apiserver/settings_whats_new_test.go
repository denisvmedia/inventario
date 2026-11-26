package apiserver_test

import (
	"encoding/json"
	"net/http"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/apiserver"
)

// The last release a user has seen on the What's new page (#1386).
func TestSettings_WhatsNewLastSeenVersion(t *testing.T) {
	c := qt.New(t)
	params, testUser, testGroup := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	path := "/api/v1/g/" + testGroup.Slug + "/settings"
	field := path + "/whats_new.last_seen_version"

	lastSeen := func() (any, bool) {
		rr := doJSON(c, handler, testUser.ID, http.MethodGet, path, "")
		c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
		var raw map[string]any
		c.Assert(json.Unmarshal(rr.Body.Bytes(), &raw), qt.IsNil)
		value, ok := raw["whatsNewLastSeenVersion"]
		return value, ok
	}

	// Never opened: the response leaves the field out.
	_, has := lastSeen()
	c.Assert(has, qt.IsFalse)

	rr := doJSON(c, handler, testUser.ID, http.MethodPatch, field, `"0.2.2"`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	value, has := lastSeen()
	c.Assert(has, qt.IsTrue)
	c.Assert(value, qt.Equals, "0.2.2")

	for _, body := range []string{`"v0.3.0"`, `"edge"`, `""`, `"0.3"`, `3`, `["0.3.0"]`} {
		rr := doJSON(c, handler, testUser.ID, http.MethodPatch, field, body)
		c.Check(rr.Code, qt.Equals, http.StatusBadRequest, qt.Commentf("%s: %s", body, rr.Body.String()))
	}
	// A refused value leaves the stored one alone.
	value, _ = lastSeen()
	c.Assert(value, qt.Equals, "0.2.2")

	rr = doJSON(c, handler, testUser.ID, http.MethodPatch, field, `{"value": "0.3.0-rc.1"}`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	value, _ = lastSeen()
	c.Assert(value, qt.Equals, "0.3.0-rc.1")

	// null forgets it, so the page counts as never opened.
	rr = doJSON(c, handler, testUser.ID, http.MethodPatch, field, `null`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	_, has = lastSeen()
	c.Assert(has, qt.IsFalse)
}
