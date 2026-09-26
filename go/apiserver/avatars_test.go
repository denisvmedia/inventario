package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/appctx"
	_ "go.5x5.cz/inventario/internal/fileblob" // registers the file:// driver
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services"
)

// #1382: the avatar endpoints. The read is mounted twice — on /auth/me for the
// owner and inside the group subtree for fellow members — and these cover the
// owner half plus the shapes a client can get wrong.

// avatarPNG encodes a small PNG for upload.
func avatarPNG(c *qt.C, size int) []byte {
	c.Helper()
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	c.Assert(png.Encode(&buf, img), qt.IsNil)
	return buf.Bytes()
}

// multipartAvatar builds a multipart body with the given field name and bytes.
func multipartAvatar(c *qt.C, field string, body []byte) (contentType string, out *bytes.Buffer) {
	c.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(field, "photo.png")
	c.Assert(err, qt.IsNil)
	_, err = part.Write(body)
	c.Assert(err, qt.IsNil)
	c.Assert(w.Close(), qt.IsNil)
	return w.FormDataContentType(), &buf
}

// avatarHandler mounts the owner-side routes with a real service over a local
// bucket, and returns a caller that runs a request as the given user.
func avatarHandler(c *qt.C) (call func(method, target, contentType string, body *bytes.Buffer) *httptest.ResponseRecorder, userID string) {
	c.Helper()

	params, user, _ := newParams()
	svc := services.NewAvatarService(params.FactorySet, "file://"+c.TempDir()+"?create_dir=1")
	api := apiserver.NewAvatarsAPIForTest(svc)

	r := chi.NewRouter()
	r.Post("/me/avatar", api.Upload)
	r.Delete("/me/avatar", api.Delete)
	r.Get("/me/avatar", api.GetOwn)

	return func(method, target, contentType string, body *bytes.Buffer) *httptest.ResponseRecorder {
		var req *http.Request
		if body == nil {
			req = httptest.NewRequest(method, target, nil)
		} else {
			req = httptest.NewRequest(method, target, bytes.NewReader(body.Bytes()))
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		req = req.WithContext(appctx.WithUser(req.Context(), user))
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		return rr
	}, user.ID
}

func TestAvatarEndpoints_UploadReadDelete(t *testing.T) {
	c := qt.New(t)
	call, _ := avatarHandler(c)

	// Nothing uploaded yet.
	c.Run("reading before there is one is a 404", func(c *qt.C) {
		rr := call(http.MethodGet, "/me/avatar", "", nil)
		c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
	})

	var stored string
	c.Run("upload returns the stored path", func(c *qt.C) {
		ct, body := multipartAvatar(c, "avatar", avatarPNG(c, 600))
		rr := call(http.MethodPost, "/me/avatar", ct, body)
		c.Assert(rr.Code, qt.Equals, http.StatusOK)

		var payload map[string]string
		c.Assert(json.Unmarshal(rr.Body.Bytes(), &payload), qt.IsNil)
		stored = payload["avatar_path"]
		c.Assert(stored, qt.Contains, "avatars/")
	})

	c.Run("the image comes back as a cacheable JPEG", func(c *qt.C) {
		rr := call(http.MethodGet, "/me/avatar", "", nil)
		c.Assert(rr.Code, qt.Equals, http.StatusOK)
		c.Assert(rr.Header().Get("Content-Type"), qt.Equals, "image/jpeg")
		// Private, because the URL is only reachable with the caller's session.
		c.Assert(rr.Header().Get("Cache-Control"), qt.Contains, "private")

		_, format, err := image.Decode(bytes.NewReader(rr.Body.Bytes()))
		c.Assert(err, qt.IsNil)
		c.Assert(format, qt.Equals, "jpeg")
	})

	c.Run("delete answers 204 and the read goes back to 404", func(c *qt.C) {
		rr := call(http.MethodDelete, "/me/avatar", "", nil)
		c.Assert(rr.Code, qt.Equals, http.StatusNoContent)

		rr = call(http.MethodGet, "/me/avatar", "", nil)
		c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
	})

	c.Run("deleting again is still 204", func(c *qt.C) {
		rr := call(http.MethodDelete, "/me/avatar", "", nil)
		c.Assert(rr.Code, qt.Equals, http.StatusNoContent)
	})
}

// The refusals a client can provoke have to be 400s, not 500s: they are the
// client's mistake and the message has to say which one.
func TestAvatarEndpoints_BadRequests(t *testing.T) {
	c := qt.New(t)
	call, _ := avatarHandler(c)

	c.Run("a body that is not multipart", func(c *qt.C) {
		rr := call(http.MethodPost, "/me/avatar", "application/json", bytes.NewBufferString(`{}`))
		c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)
	})

	c.Run("multipart with the wrong field name", func(c *qt.C) {
		ct, body := multipartAvatar(c, "photo", avatarPNG(c, 100))
		rr := call(http.MethodPost, "/me/avatar", ct, body)
		c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)
		c.Assert(rr.Body.String(), qt.Contains, "avatar")
	})

	c.Run("a file that is not an image", func(c *qt.C) {
		ct, body := multipartAvatar(c, "avatar", bytes.Repeat([]byte("not an image "), 100))
		rr := call(http.MethodPost, "/me/avatar", ct, body)
		c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)
		c.Assert(rr.Body.String(), qt.Contains, "not a supported image")
	})

	// Two different paths produce the same answer, and both have to say the
	// same thing. A file just over the cap is refused by the service; a body big
	// enough to trip the request reader never reaches it, and "invalid multipart
	// body" would send the reader looking for a malformed request.
	c.Run("a file over the size cap", func(c *qt.C) {
		oversized := append(avatarPNG(c, 8), bytes.Repeat([]byte{0}, int(services.AvatarMaxUploadBytes)+1)...)
		ct, body := multipartAvatar(c, "avatar", oversized)
		rr := call(http.MethodPost, "/me/avatar", ct, body)
		c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)
		c.Assert(rr.Body.String(), qt.Contains, "2 MB")
	})

	c.Run("a body big enough to trip the request reader", func(c *qt.C) {
		huge := append(avatarPNG(c, 8), bytes.Repeat([]byte{0}, int(services.AvatarMaxUploadBytes)+(256<<10))...)
		ct, body := multipartAvatar(c, "avatar", huge)
		rr := call(http.MethodPost, "/me/avatar", ct, body)
		c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)
		c.Assert(rr.Body.String(), qt.Contains, "2 MB",
			qt.Commentf("an oversized body was reported as a malformed one: %s", rr.Body.String()))
	})
}

// Every route requires a user in the context. Without one they must refuse
// rather than operating on an empty id.
func TestAvatarEndpoints_RequireAuthentication(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams()
	svc := services.NewAvatarService(params.FactorySet, "file://"+c.TempDir()+"?create_dir=1")
	api := apiserver.NewAvatarsAPIForTest(svc)

	r := chi.NewRouter()
	r.Post("/me/avatar", api.Upload)
	r.Delete("/me/avatar", api.Delete)
	r.Get("/me/avatar", api.GetOwn)

	for _, tc := range []struct{ method, target string }{
		{http.MethodPost, "/me/avatar"},
		{http.MethodDelete, "/me/avatar"},
		{http.MethodGet, "/me/avatar"},
	} {
		c.Run(tc.method, func(c *qt.C) {
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, httptest.NewRequest(tc.method, tc.target, nil))
			c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
		})
	}
}

// The group subtree's middleware checks that the CALLER is a member. It says
// nothing about the user id in the path, so the handler has to check that too —
// otherwise any group member could read any user's photo in the deployment by
// putting their id in the URL.
func TestAvatarEndpoints_MemberReadChecksTheSubjectsMembership(t *testing.T) {
	c := qt.New(t)

	params, caller, group := newParams()
	svc := services.NewAvatarService(params.FactorySet, "file://"+c.TempDir()+"?create_dir=1")
	groupService := services.NewGroupService(
		params.FactorySet.LocationGroupRegistry,
		params.FactorySet.GroupMembershipRegistry,
		params.FactorySet.GroupInviteRegistry,
	)
	api := apiserver.NewMemberAvatarsAPIForTest(svc, groupService)

	// An outsider in the same tenant, with an avatar, and not in the group.
	outsider, err := params.FactorySet.UserRegistry.Create(context.Background(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: caller.TenantID},
		Email:               "outsider@example.com",
		Name:                "Outsider",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	_, err = svc.Store(context.Background(), outsider.ID, bytes.NewReader(avatarPNG(c, 200)))
	c.Assert(err, qt.IsNil)

	// The caller is a member and also has one.
	_, err = svc.Store(context.Background(), caller.ID, bytes.NewReader(avatarPNG(c, 200)))
	c.Assert(err, qt.IsNil)

	r := chi.NewRouter()
	r.Get("/g/{groupID}/members/{memberUserID}/avatar",
		apiserver.WithGroupForTest(group, api.GetMember))

	get := func(userID string) int {
		req := httptest.NewRequest(http.MethodGet, "/g/"+group.ID+"/members/"+userID+"/avatar", nil)
		req = req.WithContext(appctx.WithUser(req.Context(), caller))
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		return rr.Code
	}

	c.Run("a member of the group is served", func(c *qt.C) {
		c.Assert(get(caller.ID), qt.Equals, http.StatusOK)
	})

	// 404 rather than 403: whether that user exists is not something this route
	// should confirm to somebody who shares no group with them.
	c.Run("a user outside the group is not, even though they have an avatar", func(c *qt.C) {
		c.Assert(get(outsider.ID), qt.Equals, http.StatusNotFound)
	})

	c.Run("an id that is nobody is also a 404", func(c *qt.C) {
		c.Assert(get("does-not-exist"), qt.Equals, http.StatusNotFound)
	})
}
