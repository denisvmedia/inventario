package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services"
)

// An <img> cannot send the bearer token, so the browser loads profile photos
// from the signed avatar_url the API hands out (#1382). These drive the real
// router: a URL a response carries has to be one the signed route serves, and
// the route has to refuse it once the viewer may no longer see the photo.

type signedAvatarEnv struct {
	params  apiserver.Params
	handler http.Handler
	signer  *services.FileSigningService
	avatars *services.AvatarService
	group   *models.LocationGroup

	owner    *models.User // in the group, has a photo
	member   *models.User // in the group, has a photo
	plain    *models.User // in the group, no photo
	stranger *models.User // same tenant, not in the group, has a photo
}

func newSignedAvatarEnv(c *qt.C) *signedAvatarEnv {
	c.Helper()
	params, owner, group := newParams()
	params.UploadLocation = "file://" + c.TempDir() + "?create_dir=1"
	ctx := context.Background()

	join := func(email string) *models.User {
		u := createTestUserDirect(c, params, owner.TenantID, email, true, false)
		must.Must(params.FactorySet.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: owner.TenantID},
			GroupID:             group.ID,
			MemberUserID:        u.ID,
			Role:                models.GroupRoleViewer,
		}))
		return u
	}

	env := &signedAvatarEnv{
		params:   params,
		handler:  apiserver.APIServer(params, &mockRestoreWorker{}),
		signer:   services.NewFileSigningService(params.FileSigningKey, params.FileURLExpiration),
		avatars:  services.NewAvatarService(params.FactorySet, params.UploadLocation),
		group:    group,
		owner:    owner,
		member:   join("photo-member@example.com"),
		plain:    join("no-photo@example.com"),
		stranger: createTestUserDirect(c, params, owner.TenantID, "photo-stranger@example.com", true, false),
	}
	for _, u := range []*models.User{env.owner, env.member, env.stranger} {
		_, err := env.avatars.Store(ctx, u.ID, bytes.NewReader(avatarPNG(c, 120)))
		c.Assert(err, qt.IsNil)
	}
	return env
}

// api runs an authenticated API request.
func (e *signedAvatarEnv) api(c *qt.C, path string, user *models.User) *httptest.ResponseRecorder {
	c.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+createTestJWTToken(user.ID))
	rr := httptest.NewRecorder()
	e.handler.ServeHTTP(rr, req)
	return rr
}

// image fetches a signed URL the way an <img> does: no bearer token.
func (e *signedAvatarEnv) image(signed string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, signed, nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()
	e.handler.ServeHTTP(rr, req)
	return rr
}

// mint signs a URL directly, standing in for a response that handed one out.
func (e *signedAvatarEnv) mint(c *qt.C, subject, viewer *models.User, groupID string) string {
	c.Helper()
	current := must.Must(e.params.FactorySet.UserRegistry.Get(context.Background(), subject.ID))
	c.Assert(current.AvatarPath, qt.IsNotNil)
	signed, err := e.signer.GenerateAvatarURL(subject.ID, *current.AvatarPath, viewer.ID, groupID, "")
	c.Assert(err, qt.IsNil)
	return signed
}

func assertServesJPEG(c *qt.C, rr *httptest.ResponseRecorder) {
	c.Helper()
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", rr.Body.String()))
	c.Assert(rr.Header().Get("Content-Type"), qt.Equals, "image/jpeg")
	_, format, err := image.Decode(bytes.NewReader(rr.Body.Bytes()))
	c.Assert(err, qt.IsNil)
	c.Assert(format, qt.Equals, "jpeg")
}

func TestSignedAvatar_AuthMeCarriesAURLTheRouteServes(t *testing.T) {
	c := qt.New(t)
	env := newSignedAvatarEnv(c)

	rr := env.api(c, "/api/v1/auth/me", env.owner)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	var me models.User
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &me), qt.IsNil)
	c.Assert(strings.HasPrefix(me.AvatarURL, services.AvatarURLPrefix+env.owner.ID+"?"), qt.IsTrue, qt.Commentf("got %q", me.AvatarURL))

	assertServesJPEG(c, env.image(me.AvatarURL))

	c.Run("a user without a photo gets no URL", func(c *qt.C) {
		rr := env.api(c, "/api/v1/auth/me", env.plain)
		c.Assert(rr.Code, qt.Equals, http.StatusOK)
		c.Assert(rr.Body.String(), qt.Not(qt.Contains), "avatar_url")
	})
}

func TestSignedAvatar_MembersListCarriesURLs(t *testing.T) {
	c := qt.New(t)
	env := newSignedAvatarEnv(c)

	rr := env.api(c, "/api/v1/groups/"+env.group.ID+"/members", env.member)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var body struct {
		Data []struct {
			Attributes struct {
				User struct {
					ID        string `json:"id"`
					AvatarURL string `json:"avatar_url"`
				} `json:"user"`
			} `json:"attributes"`
		} `json:"data"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)

	urls := map[string]string{}
	for _, row := range body.Data {
		urls[row.Attributes.User.ID] = row.Attributes.User.AvatarURL
	}
	c.Assert(urls, qt.HasLen, 3)
	c.Assert(urls[env.plain.ID], qt.Equals, "")
	for _, u := range []*models.User{env.owner, env.member} {
		c.Assert(urls[u.ID], qt.Not(qt.Equals), "", qt.Commentf("no URL for %s", u.Email))
		assertServesJPEG(c, env.image(urls[u.ID]))
	}

	// A member's own row gets the URL /auth/me gives them, so the browser
	// loads their photo once for the sidebar and the list.
	rr = env.api(c, "/api/v1/auth/me", env.member)
	var me models.User
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &me), qt.IsNil)
	c.Assert(urls[env.member.ID], qt.Equals, me.AvatarURL)
}

// The URL records a decision made when it was minted. Each case here is a way
// that decision stops being true before the URL expires.
func TestSignedAvatar_ChecksAgainWhenServing(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	c.Run("a fellow member is served", func(c *qt.C) {
		env := newSignedAvatarEnv(c)
		assertServesJPEG(c, env.image(env.mint(c, env.owner, env.member, env.group.ID)))
	})

	c.Run("a viewer who is not in the group is not", func(c *qt.C) {
		env := newSignedAvatarEnv(c)
		rr := env.image(env.mint(c, env.owner, env.stranger, env.group.ID))
		c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
	})

	c.Run("a subject who is not in the group is not", func(c *qt.C) {
		env := newSignedAvatarEnv(c)
		rr := env.image(env.mint(c, env.stranger, env.member, env.group.ID))
		c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
	})

	c.Run("a member who has left the group is no longer served", func(c *qt.C) {
		env := newSignedAvatarEnv(c)
		signed := env.mint(c, env.owner, env.member, env.group.ID)
		assertServesJPEG(c, env.image(signed))

		membership := must.Must(env.params.FactorySet.GroupMembershipRegistry.GetByGroupAndUser(ctx, env.group.ID, env.member.ID))
		c.Assert(env.params.FactorySet.GroupMembershipRegistry.Delete(ctx, membership.ID), qt.IsNil)
		c.Assert(env.image(signed).Code, qt.Equals, http.StatusNotFound)
	})

	c.Run("a disabled viewer is refused", func(c *qt.C) {
		env := newSignedAvatarEnv(c)
		signed := env.mint(c, env.owner, env.member, env.group.ID)
		member := must.Must(env.params.FactorySet.UserRegistry.Get(ctx, env.member.ID))
		member.IsActive = false
		must.Must(env.params.FactorySet.UserRegistry.Update(ctx, *member))
		c.Assert(env.image(signed).Code, qt.Equals, http.StatusUnauthorized)
	})

	c.Run("a replaced photo's old URL is not served", func(c *qt.C) {
		env := newSignedAvatarEnv(c)
		old := env.mint(c, env.owner, env.member, env.group.ID)
		_, err := env.avatars.Store(ctx, env.owner.ID, bytes.NewReader(avatarPNG(c, 90)))
		c.Assert(err, qt.IsNil)
		c.Assert(env.image(old).Code, qt.Equals, http.StatusNotFound)
		assertServesJPEG(c, env.image(env.mint(c, env.owner, env.member, env.group.ID)))
	})

	c.Run("a removed photo is not served", func(c *qt.C) {
		env := newSignedAvatarEnv(c)
		signed := env.mint(c, env.owner, env.owner, "")
		c.Assert(env.avatars.Remove(ctx, env.owner.ID), qt.IsNil)
		c.Assert(env.image(signed).Code, qt.Equals, http.StatusNotFound)
	})

	c.Run("a URL presented from another session is refused", func(c *qt.C) {
		env := newSignedAvatarEnv(c)
		signed := env.mint(c, env.owner, env.owner, "")
		rr := env.image(signed, &http.Cookie{
			Name:     appctx.RefreshTokenCookieName,
			Value:    "someone-elses-session",
			Path:     "/api/v1",
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
		c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
	})

	c.Run("a URL with the viewer swapped is refused", func(c *qt.C) {
		env := newSignedAvatarEnv(c)
		signed := env.mint(c, env.owner, env.member, env.group.ID)
		forged := strings.Replace(signed, "uid="+env.member.ID, "uid="+env.plain.ID, 1)
		c.Assert(forged, qt.Not(qt.Equals), signed)
		c.Assert(env.image(forged).Code, qt.Equals, http.StatusUnauthorized)
	})
}

// adminAPI runs a back-office request as the operator token names.
func (e *signedAvatarEnv) adminAPI(c *qt.C, path, token string) *httptest.ResponseRecorder {
	c.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	addBackofficeAuthHeader(req, token)
	rr := httptest.NewRecorder()
	e.handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("%s: %s", path, rr.Body.String()))
	return rr
}

// The back-office user list, user detail and group members responses carry
// a photo URL signed for the operator, and the signed route serves it
// (#2894).
func TestSignedAvatar_AdminResponsesCarryURLsTheRouteServes(t *testing.T) {
	c := qt.New(t)
	env := newSignedAvatarEnv(c)
	_, token := WithBackofficeAdmin(t, env.params)

	var list struct {
		Data []struct {
			ID        string `json:"id"`
			AvatarURL string `json:"avatar_url"`
		} `json:"data"`
	}
	rr := env.adminAPI(c, "/api/v1/admin/tenants/"+env.owner.TenantID+"/users?per_page=100", token)
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &list), qt.IsNil)
	listed := map[string]string{}
	for _, row := range list.Data {
		listed[row.ID] = row.AvatarURL
	}
	c.Assert(listed[env.plain.ID], qt.Equals, "")
	for _, u := range []*models.User{env.owner, env.member, env.stranger} {
		c.Assert(listed[u.ID], qt.Not(qt.Equals), "", qt.Commentf("no list URL for %s", u.Email))
		assertServesJPEG(c, env.image(listed[u.ID]))
	}

	var detail struct {
		Data struct {
			AvatarURL string `json:"avatar_url"`
		} `json:"data"`
	}
	rr = env.adminAPI(c, "/api/v1/admin/users/"+env.stranger.ID, token)
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &detail), qt.IsNil)
	assertServesJPEG(c, env.image(detail.Data.AvatarURL))

	var members struct {
		Data []struct {
			User struct {
				ID        string `json:"id"`
				AvatarURL string `json:"avatar_url"`
			} `json:"user"`
		} `json:"data"`
	}
	rr = env.adminAPI(c, "/api/v1/admin/groups/"+env.group.ID+"/members", token)
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &members), qt.IsNil)
	memberURLs := map[string]string{}
	for _, row := range members.Data {
		memberURLs[row.User.ID] = row.User.AvatarURL
	}
	c.Assert(memberURLs, qt.HasLen, 3)
	c.Assert(memberURLs[env.plain.ID], qt.Equals, "")
	assertServesJPEG(c, env.image(memberURLs[env.member.ID]))
}

// An operator's URL is checked against the operator when it is served: a
// disabled operator's URLs stop working, and a user cannot use one by
// naming themselves in it.
func TestSignedAvatar_OperatorURLChecksTheOperatorAgain(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	mintFor := func(c *qt.C, env *signedAvatarEnv, operatorID string) string {
		current := must.Must(env.params.FactorySet.UserRegistry.Get(ctx, env.stranger.ID))
		signed, err := env.signer.GenerateOperatorAvatarURL(current.ID, *current.AvatarPath, operatorID, "")
		c.Assert(err, qt.IsNil)
		return signed
	}

	c.Run("an active operator is served", func(c *qt.C) {
		env := newSignedAvatarEnv(c)
		operator, _ := WithBackofficeAdmin(t, env.params)
		assertServesJPEG(c, env.image(mintFor(c, env, operator.ID)))
	})

	c.Run("a disabled operator is refused", func(c *qt.C) {
		env := newSignedAvatarEnv(c)
		operator, _ := WithBackofficeAdmin(t, env.params)
		signed := mintFor(c, env, operator.ID)
		c.Assert(env.params.FactorySet.BackofficeUserRegistry.SetActive(ctx, operator.ID, false), qt.IsNil)
		c.Assert(env.image(signed).Code, qt.Equals, http.StatusUnauthorized)
	})

	c.Run("a tenant user named as the operator is refused", func(c *qt.C) {
		env := newSignedAvatarEnv(c)
		c.Assert(env.image(mintFor(c, env, env.member.ID)).Code, qt.Equals, http.StatusUnauthorized)
	})
}

func TestAvatarEndpoints_UploadAnswersWithASignedURL(t *testing.T) {
	c := qt.New(t)
	params, user, _ := newParams()
	svc := services.NewAvatarService(params.FactorySet, "file://"+c.TempDir()+"?create_dir=1")
	signer := services.NewFileSigningService(params.FileSigningKey, params.FileURLExpiration)
	api := apiserver.NewAvatarsAPIWithSignerForTest(svc, signer)

	r := chi.NewRouter()
	r.Post("/me/avatar", api.Upload)

	ct, body := multipartAvatar(c, "avatar", avatarPNG(c, 300))
	req := httptest.NewRequest(http.MethodPost, "/me/avatar", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", ct)
	req = req.WithContext(appctx.WithUser(req.Context(), user))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var payload apiserver.AvatarUploadResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &payload), qt.IsNil)
	c.Assert(payload.AvatarPath, qt.Contains, "avatars/"+user.ID+"/")
	c.Assert(strings.HasPrefix(payload.AvatarURL, services.AvatarURLPrefix+user.ID+"?"), qt.IsTrue, qt.Commentf("got %q", payload.AvatarURL))
	c.Assert(payload.AvatarURL, qt.Contains, "v="+services.AvatarVersion(payload.AvatarPath))
}
