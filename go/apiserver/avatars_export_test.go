package apiserver

import (
	"context"
	"net/http"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services"
)

// AvatarsAPIForTest exposes the avatar handlers to the black-box test package.
// The routes themselves are mounted inside Auth() and Groups(), which need a
// whole server's worth of wiring to exercise one handler.
type AvatarsAPIForTest struct{ api *avatarsAPI }

// NewAvatarsAPIForTest builds the handler set over the given service. It
// compiles only under `go test`.
func NewAvatarsAPIForTest(svc *services.AvatarService) *AvatarsAPIForTest {
	return &AvatarsAPIForTest{api: &avatarsAPI{avatarService: svc}}
}

func (a *AvatarsAPIForTest) Upload(w http.ResponseWriter, r *http.Request) {
	a.api.handleUploadAvatar(w, r)
}

func (a *AvatarsAPIForTest) Delete(w http.ResponseWriter, r *http.Request) {
	a.api.handleDeleteAvatar(w, r)
}

func (a *AvatarsAPIForTest) GetOwn(w http.ResponseWriter, r *http.Request) {
	a.api.handleGetOwnAvatar(w, r)
}

func (a *AvatarsAPIForTest) GetMember(w http.ResponseWriter, r *http.Request) {
	a.api.handleGetMemberAvatar(w, r)
}

// WithGroupForTest returns a handler that runs next with the given group in the
// request context, standing in for the groupCtx middleware the real routes use.
// It compiles only under `go test`.
func WithGroupForTest(group *models.LocationGroup, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next(w, r.WithContext(context.WithValue(r.Context(), groupCtxKey, group)))
	}
}

// NewMemberAvatarsAPIForTest builds the handler set with the group service the
// member route needs to check the subject's membership.
func NewMemberAvatarsAPIForTest(svc *services.AvatarService, groupService *services.GroupService) *AvatarsAPIForTest {
	return &AvatarsAPIForTest{api: &avatarsAPI{avatarService: svc, groupService: groupService}}
}
