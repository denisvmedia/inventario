package services_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/services"
)

const avatarTestKey = "avatars/subject-1/3-a1b2c3d4e5f6.jpg"

func newAvatarSigner() *services.FileSigningService {
	return services.NewFileSigningService([]byte("avatar-signing-key-32-bytes!!!!!"), time.Hour)
}

// splitSignedURL returns the path and query of a signed URL.
func splitSignedURL(c *qt.C, signed string) (string, url.Values) {
	c.Helper()
	u, err := url.Parse(signed)
	c.Assert(err, qt.IsNil)
	return u.Path, u.Query()
}

func TestAvatarURL_RoundTrip(t *testing.T) {
	c := qt.New(t)
	signer := newAvatarSigner()

	c.Run("another member's photo carries the group", func(c *qt.C) {
		signed, err := signer.GenerateAvatarURL("subject-1", avatarTestKey, "viewer-1", "group-1", "binding")
		c.Assert(err, qt.IsNil)
		c.Assert(strings.HasPrefix(signed, services.AvatarURLPrefix+"subject-1?"), qt.IsTrue, qt.Commentf("got %s", signed))

		urlPath, query := splitSignedURL(c, signed)
		claims, err := signer.ValidateAvatarURL(urlPath, query, "binding")
		c.Assert(err, qt.IsNil)
		c.Assert(claims.SubjectID, qt.Equals, "subject-1")
		c.Assert(claims.ViewerID, qt.Equals, "viewer-1")
		c.Assert(claims.GroupID, qt.Equals, "group-1")
		c.Assert(claims.Version, qt.Equals, "3-a1b2c3d4e5f6")
		c.Assert(claims.ExpiresAt.After(time.Now().Add(time.Hour-time.Second)), qt.IsTrue)
	})

	c.Run("your own photo needs no group", func(c *qt.C) {
		signed, err := signer.GenerateAvatarURL("subject-1", avatarTestKey, "subject-1", "", "")
		c.Assert(err, qt.IsNil)
		urlPath, query := splitSignedURL(c, signed)
		claims, err := signer.ValidateAvatarURL(urlPath, query, "")
		c.Assert(err, qt.IsNil)
		c.Assert(claims.GroupID, qt.Equals, "")
	})
}

// Two responses within one period hand out the same URL, so the browser serves
// the second from its cache instead of fetching the photo again.
func TestAvatarURL_IsStableWithinAPeriod(t *testing.T) {
	c := qt.New(t)
	signer := newAvatarSigner()

	first, err := signer.GenerateAvatarURL("subject-1", avatarTestKey, "viewer-1", "group-1", "binding")
	c.Assert(err, qt.IsNil)
	second, err := signer.GenerateAvatarURL("subject-1", avatarTestKey, "viewer-1", "group-1", "binding")
	c.Assert(err, qt.IsNil)
	c.Assert(second, qt.Equals, first)

	// A replaced photo is a new URL.
	replaced, err := signer.GenerateAvatarURL("subject-1", "avatars/subject-1/4-ffffffffffff.jpg", "viewer-1", "group-1", "binding")
	c.Assert(err, qt.IsNil)
	c.Assert(replaced, qt.Not(qt.Equals), first)
}

// Every part the URL carries is covered by the signature: changing any of them
// is refused, as is presenting the URL from another session.
func TestAvatarURL_RefusesTampering(t *testing.T) {
	c := qt.New(t)
	signer := newAvatarSigner()
	signed, err := signer.GenerateAvatarURL("subject-1", avatarTestKey, "viewer-1", "group-1", "binding")
	c.Assert(err, qt.IsNil)
	urlPath, query := splitSignedURL(c, signed)

	cases := []struct {
		name    string
		path    string
		mutate  func(q url.Values)
		binding services.SessionBinding
	}{
		{name: "another subject", path: services.AvatarURLPrefix + "subject-2"},
		{name: "another viewer", mutate: func(q url.Values) { q.Set("uid", "viewer-2") }},
		{name: "another group", mutate: func(q url.Values) { q.Set("gid", "group-2") }},
		{name: "the group dropped", mutate: func(q url.Values) { q.Del("gid") }},
		{name: "another version", mutate: func(q url.Values) { q.Set("v", "4-ffffffffffff") }},
		{name: "a later expiry", mutate: func(q url.Values) { q.Set("exp", "99999999999") }},
		{name: "an expiry in the past", mutate: func(q url.Values) { q.Set("exp", "1") }},
		{name: "no signature", mutate: func(q url.Values) { q.Del("sig") }},
		{name: "another session", binding: "other-binding"},
		{name: "a nested path", path: services.AvatarURLPrefix + "subject-1/extra"},
		{name: "a file path", path: "/api/v1/files/download/files/subject-1"},
	}
	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			q := url.Values{}
			for k, v := range query {
				q[k] = append([]string(nil), v...)
			}
			if tc.mutate != nil {
				tc.mutate(q)
			}
			p := urlPath
			if tc.path != "" {
				p = tc.path
			}
			binding := services.SessionBinding("binding")
			if tc.binding != "" {
				binding = tc.binding
			}
			_, err := signer.ValidateAvatarURL(p, q, binding)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

func TestAvatarURL_RefusesToSignWithoutTheGroupItNeeds(t *testing.T) {
	c := qt.New(t)
	signer := newAvatarSigner()

	_, err := signer.GenerateAvatarURL("subject-1", avatarTestKey, "viewer-1", "", "")
	c.Assert(err, qt.IsNotNil)

	_, err = signer.GenerateAvatarURL("subject-1", "", "subject-1", "", "")
	c.Assert(err, qt.IsNotNil)
}

// A back-office operator's URL names the operator instead of a viewer
// (#2894). Its signature covers a message of its own, so it cannot be turned
// into a user URL, or the other way round.
func TestOperatorAvatarURL_RoundTrip(t *testing.T) {
	c := qt.New(t)
	signer := newAvatarSigner()

	signed, err := signer.GenerateOperatorAvatarURL("subject-1", avatarTestKey, "operator-1", "binding")
	c.Assert(err, qt.IsNil)
	c.Assert(strings.HasPrefix(signed, services.AvatarURLPrefix+"subject-1?"), qt.IsTrue, qt.Commentf("got %s", signed))

	urlPath, query := splitSignedURL(c, signed)
	claims, err := signer.ValidateAvatarURL(urlPath, query, "binding")
	c.Assert(err, qt.IsNil)
	c.Assert(claims.SubjectID, qt.Equals, "subject-1")
	c.Assert(claims.OperatorID, qt.Equals, "operator-1")
	c.Assert(claims.ViewerID, qt.Equals, "")
	c.Assert(claims.GroupID, qt.Equals, "")
	c.Assert(claims.Version, qt.Equals, "3-a1b2c3d4e5f6")
}

func TestOperatorAvatarURL_RefusesTampering(t *testing.T) {
	c := qt.New(t)
	signer := newAvatarSigner()
	operatorURL, err := signer.GenerateOperatorAvatarURL("subject-1", avatarTestKey, "operator-1", "binding")
	c.Assert(err, qt.IsNil)
	userURL, err := signer.GenerateAvatarURL("subject-1", avatarTestKey, "viewer-1", "group-1", "binding")
	c.Assert(err, qt.IsNil)

	cases := []struct {
		name   string
		signed string
		mutate func(q url.Values)
	}{
		{name: "another operator", signed: operatorURL, mutate: func(q url.Values) { q.Set("op", "operator-2") }},
		{name: "the operator passed off as a user", signed: operatorURL, mutate: func(q url.Values) {
			q.Set("uid", q.Get("op"))
			q.Del("op")
		}},
		{name: "a user passed off as an operator", signed: userURL, mutate: func(q url.Values) {
			q.Set("op", q.Get("uid"))
			q.Del("uid")
			q.Del("gid")
		}},
		{name: "a group added", signed: operatorURL, mutate: func(q url.Values) { q.Set("gid", "group-1") }},
		{name: "a user added", signed: operatorURL, mutate: func(q url.Values) { q.Set("uid", "viewer-1") }},
		{name: "an operator added to a user URL", signed: userURL, mutate: func(q url.Values) { q.Set("op", "operator-1") }},
		{name: "another session", signed: operatorURL},
	}
	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			urlPath, query := splitSignedURL(c, tc.signed)
			binding := services.SessionBinding("other-binding")
			if tc.mutate != nil {
				tc.mutate(query)
				binding = "binding"
			}
			_, err := signer.ValidateAvatarURL(urlPath, query, binding)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

func TestOperatorAvatarURL_RefusesToSignWithoutAnOperator(t *testing.T) {
	c := qt.New(t)
	signer := newAvatarSigner()

	_, err := signer.GenerateOperatorAvatarURL("subject-1", avatarTestKey, "", "")
	c.Assert(err, qt.IsNotNil)

	_, err = signer.GenerateOperatorAvatarURL("subject-1", "", "operator-1", "")
	c.Assert(err, qt.IsNotNil)
}
