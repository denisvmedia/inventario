package apiserver_test

import (
	"bytes"
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

	c.Run("an image over the size cap", func(c *qt.C) {
		oversized := append(avatarPNG(c, 8), bytes.Repeat([]byte{0}, int(services.AvatarMaxUploadBytes)+1)...)
		ct, body := multipartAvatar(c, "avatar", oversized)
		rr := call(http.MethodPost, "/me/avatar", ct, body)
		c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)
		c.Assert(rr.Body.String(), qt.Contains, "2 MB")
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
