package apiserver

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// avatarsAPI serves the profile photo endpoints (#1382).
//
// Reads are mounted twice: once on /auth/me for the owner and once inside the
// group subtree for everybody else. The rule is "you can see the avatar of
// someone you share a group with", and it takes two checks, not one: the
// subtree's requireGroupMember middleware covers the CALLER, and
// handleGetMemberAvatar checks the SUBJECT. Without the second the path
// parameter would be an unchecked user id, and any group member could read any
// user's photo in the deployment.
//
// Those two reads need the bearer token, which an <img> does not send. The
// browser loads photos from signed URLs instead (signedAvatarsAPI), which the
// /auth/me and members responses carry as avatar_url.
type avatarsAPI struct {
	avatarService *services.AvatarService
	// groupService answers whether the path's user is in the context group.
	groupService *services.GroupService
	// signer mints the avatar_url the upload response carries. Nil leaves it
	// out.
	signer *services.FileSigningService
}

// avatarMultipartMemory is how much of a multipart body is buffered in memory
// before the rest spills to a temporary file. The whole upload is capped well
// below it, so in practice nothing spills.
const (
	avatarMultipartMemory = int64(services.AvatarMaxUploadBytes) + 1024
	// avatarMaxBodyBytes caps the whole request, leaving room for the multipart
	// framing around a file that is itself capped at AvatarMaxUploadBytes. The
	// service still checks the file, so this is the outer guard rather than the
	// rule.
	avatarMaxBodyBytes = int64(services.AvatarMaxUploadBytes) + (64 << 10)
)

// handleUploadAvatar replaces the authenticated user's profile photo.
//
// @Summary Upload a profile photo
// @Description Replaces the caller's avatar. The image is center-cropped to a square, scaled to 512px and re-encoded as JPEG, which also drops any EXIF metadata the original carried.
// @Description Accepts a still JPEG, PNG or WebP up to 2 MB, detected from the content rather than from the declared type or the filename. The EXIF orientation is applied to the pixels before the metadata is dropped.
// @Tags auth
// @Accept multipart/form-data
// @Produce json
// @Param avatar formData file true "Image file"
// @Success 200 {object} AvatarUploadResponse "The stored avatar path and a signed URL to load it from"
// @Failure 400 {string} string "Not a supported image, animated, or larger than 2 MB"
// @Failure 401 {string} string "Authentication required"
// @Router /auth/me/avatar [post]
func (api *avatarsAPI) handleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// ParseMultipartForm's argument caps what is held in memory, not what is
	// read: without this the body can be any size and spills to disk. The reader
	// is the actual bound, and it is what gosec's G120 is about.
	r.Body = http.MaxBytesReader(w, r.Body, avatarMaxBodyBytes)
	// #nosec G120 -- the body is bounded by the MaxBytesReader above;
	// ParseMultipartForm's argument caps memory, not the request.
	if err := r.ParseMultipartForm(avatarMultipartMemory); err != nil {
		// The reader above trips before the file is parsed, so an oversized
		// upload arrives here rather than at the service's own size check. Say
		// the same thing either way; "invalid multipart body" would send the
		// reader looking for a malformed request.
		if _, tooLarge := errors.AsType[*http.MaxBytesError](err); tooLarge {
			http.Error(w, "The image is larger than 2 MB", http.StatusBadRequest)
			return
		}
		http.Error(w, "Invalid multipart body", http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("avatar")
	if err != nil {
		http.Error(w, "Missing the avatar file field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	path, err := api.avatarService.Store(r.Context(), user.ID, file)
	switch {
	case errors.Is(err, services.ErrAvatarTooLarge):
		http.Error(w, "The image is larger than 2 MB", http.StatusBadRequest)
		return
	case errors.Is(err, services.ErrAvatarNotAnImage):
		http.Error(w, "The file is not a supported image", http.StatusBadRequest)
		return
	case errors.Is(err, services.ErrAvatarAnimated):
		http.Error(w, "Animated images are not supported", http.StatusBadRequest)
		return
	case err != nil:
		_ = internalServerError(w, r, err)
		return
	}

	user.AvatarPath = &path
	writeAvatarUpload(w, AvatarUploadResponse{
		AvatarPath: path,
		AvatarURL:  signedAvatarURL(r, api.signer, user, user.ID, ""),
	})
}

// handleDeleteAvatar removes the authenticated user's profile photo.
//
// @Summary Remove the profile photo
// @Description Clears the caller's avatar and deletes the stored image. Idempotent: removing an avatar that is not there succeeds.
// @Tags auth
// @Produce json
// @Success 204 "Removed"
// @Failure 401 {string} string "Authentication required"
// @Router /auth/me/avatar [delete]
func (api *avatarsAPI) handleDeleteAvatar(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	if err := api.avatarService.Remove(r.Context(), user.ID); err != nil {
		_ = internalServerError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleGetOwnAvatar streams the authenticated user's profile photo.
//
// @Summary Get your own profile photo
// @Tags auth
// @Produce image/jpeg
// @Success 200 {file} file "The avatar image"
// @Failure 401 {string} string "Authentication required"
// @Failure 404 {string} string "No avatar set"
// @Router /auth/me/avatar [get]
func (api *avatarsAPI) handleGetOwnAvatar(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	api.streamAvatar(w, r, user.ID)
}

// handleGetMemberAvatar streams a fellow group member's profile photo. The group
// subtree already requires membership, so reaching this handler is the check.
//
// @Summary Get a group member's profile photo
// @Tags groups
// @Produce image/jpeg
// @Param groupID path string true "Group ID"
// @Param memberUserID path string true "Member user ID"
// @Success 200 {file} file "The avatar image"
// @Failure 403 {string} string "Not a member of this group"
// @Failure 404 {string} string "No avatar set, or not a member of this group"
// @Router /groups/{groupID}/members/{memberUserID}/avatar [get].
func (api *avatarsAPI) handleGetMemberAvatar(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		http.Error(w, "Group context required", http.StatusInternalServerError)
		return
	}

	memberUserID := chi.URLParam(r, "memberUserID")
	// The middleware on this subtree established that the CALLER is a member.
	// This establishes that the SUBJECT is, which is the half that stops the
	// path parameter being an unchecked user id.
	//
	// 404 rather than 403: whether a given user exists at all is not something
	// this route should confirm to somebody they share no group with.
	if api.groupService == nil || memberUserID == "" ||
		!api.groupService.IsGroupMember(r.Context(), group.ID, memberUserID) {
		http.Error(w, "No avatar", http.StatusNotFound)
		return
	}

	api.streamAvatar(w, r, memberUserID)
}

// streamAvatar writes the stored image, or 404 when there is none.
func (api *avatarsAPI) streamAvatar(w http.ResponseWriter, r *http.Request, userID string) {
	reader, contentType, err := api.avatarService.Open(r.Context(), userID)
	writeAvatarImage(w, r, reader, contentType, err)
}

// writeAvatarImage streams an opened avatar, or answers 404 when there is none.
func writeAvatarImage(w http.ResponseWriter, r *http.Request, reader io.ReadCloser, contentType string, err error) {
	switch {
	case errors.Is(err, registry.ErrNotFound):
		http.Error(w, "No avatar", http.StatusNotFound)
		return
	case err != nil:
		_ = internalServerError(w, r, err)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", contentType)
	// The key carries a revision, so a given URL's bytes never change and the
	// response can be cached hard. Private, because the URL is only reachable
	// with the caller's session.
	w.Header().Set("Cache-Control", "private, max-age=86400")
	if _, err := io.Copy(w, reader); err != nil {
		// The status is already written; nothing useful is left to say to the
		// client, and the transfer failing is the client's problem more often
		// than ours.
		return
	}
}

// AvatarUploadResponse answers a profile photo upload.
type AvatarUploadResponse struct {
	// AvatarPath is the stored key.
	AvatarPath string `json:"avatar_path"`
	// AvatarURL is a signed URL the browser can load the new photo from.
	// Empty when the server has no signer configured.
	AvatarURL string `json:"avatar_url,omitempty"`
}

func writeAvatarUpload(w http.ResponseWriter, body AvatarUploadResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// The status is already committed; an encoding failure has nowhere to go.
	_ = json.NewEncoder(w).Encode(body)
}

// signedAvatarURL returns a URL the viewer's browser can load the subject's
// photo from, or "" when the subject has none, or signing fails: a missing URL
// falls back to initials, which is not worth failing a response over.
//
// The caller has already established that the viewer may see the photo.
// groupID is the group the two share and is ignored when they are the same
// user, so a user's own photo gets the same URL on every surface and the
// browser loads it once.
func signedAvatarURL(r *http.Request, signer *services.FileSigningService, subject *models.User, viewerID, groupID string) string {
	if signer == nil || subject == nil || subject.AvatarPath == nil || *subject.AvatarPath == "" {
		return ""
	}
	if subject.ID == viewerID {
		groupID = ""
	}
	signed, err := signer.GenerateAvatarURL(subject.ID, *subject.AvatarPath, viewerID, groupID, services.ExtractSessionBinding(r))
	if err != nil {
		slog.WarnContext(r.Context(), "Failed to sign an avatar URL", "subject_id", subject.ID, "viewer_id", viewerID, "error", err)
		return ""
	}
	return signed
}

// stampOwnAvatarURL sets the wire-only AvatarURL on a /auth/me-style payload.
// It always assigns, so a URL left on the struct by an earlier request never
// outlives the photo it pointed at.
func stampOwnAvatarURL(r *http.Request, signer *services.FileSigningService, user *models.User) {
	if user == nil {
		return
	}
	user.AvatarURL = signedAvatarURL(r, signer, user, user.ID, "")
}

// signedAvatarsAPI serves profile photos from signed URLs. It is mounted outside
// the JWT middleware, like /files/download: an <img> does not send the bearer
// token, so the URL's HMAC is the credential.
//
// The URL proves that the server decided, when it minted it, that the viewer
// may see the subject's photo. That decision is checked again here, because a
// membership can end, or an account be disabled, before the URL expires.
type signedAvatarsAPI struct {
	signer        *services.FileSigningService
	users         registry.UserRegistry
	memberships   registry.GroupMembershipRegistry
	avatarService *services.AvatarService
}

// SignedAvatars returns the route handler for signed profile photo URLs. It
// checks the URL itself instead of going through SignedURLMiddleware, which
// resolves a file row; an avatar has none.
func SignedAvatars(params Params) func(r chi.Router) {
	api := &signedAvatarsAPI{
		signer:        services.NewFileSigningService(params.FileSigningKey, params.FileURLExpiration),
		users:         params.FactorySet.UserRegistry,
		memberships:   params.FactorySet.GroupMembershipRegistry,
		avatarService: services.NewAvatarService(params.FactorySet, params.UploadLocation),
	}
	return func(r chi.Router) {
		r.Get("/{userID}", api.handleGetSignedAvatar)
	}
}

// handleGetSignedAvatar streams a profile photo from a signed URL.
//
// @Summary Get a profile photo from a signed URL
// @Description Serves the photo a signed avatar_url points at. The /auth/me and group members responses carry those URLs;
// @Description they are bound to the session that received them and expire. The signature is the credential, so no bearer token is needed.
// @Tags auth
// @Produce image/jpeg
// @Param userID path string true "The user whose photo it is"
// @Param v query string true "Photo version"
// @Param uid query string true "The user the URL was issued to"
// @Param gid query string false "The group the two users share"
// @Param exp query int true "Expiry, Unix seconds"
// @Param sig query string true "Signature"
// @Success 200 {file} file "The avatar image"
// @Failure 401 {string} string "Invalid or expired URL"
// @Failure 404 {string} string "No such photo, or no longer visible to this user"
// @Router /avatars/{userID} [get]
func (api *signedAvatarsAPI) handleGetSignedAvatar(w http.ResponseWriter, r *http.Request) {
	claims, err := api.signer.ValidateAvatarURL(r.URL.Path, r.URL.Query(), services.ExtractSessionBinding(r))
	if err != nil {
		slog.WarnContext(r.Context(), "Invalid signed avatar URL",
			"path", r.URL.Path, "error", err.Error(), "remote_addr", r.RemoteAddr)
		http.Error(w, "Invalid or expired avatar URL", http.StatusUnauthorized)
		return
	}

	viewer, err := api.users.Get(r.Context(), claims.ViewerID)
	if err != nil || !viewer.IsActive {
		http.Error(w, "Invalid or expired avatar URL", http.StatusUnauthorized)
		return
	}

	subject, ok := api.visibleSubject(r, viewer, claims)
	// 404 for every reason the photo is not visible: whether a user exists, or
	// has a photo, is not something to confirm to somebody who may not see it.
	if !ok || subject.AvatarPath == nil || services.AvatarVersion(*subject.AvatarPath) != claims.Version {
		http.Error(w, "No avatar", http.StatusNotFound)
		return
	}

	reader, contentType, err := api.avatarService.OpenUserAvatar(r.Context(), subject)
	writeAvatarImage(w, r, reader, contentType, err)
}

// visibleSubject loads the URL's subject and checks the viewer may still see
// them: the viewer themselves, or a user in the same tenant who is, like the
// viewer, a member of the group the URL was signed for.
func (api *signedAvatarsAPI) visibleSubject(r *http.Request, viewer *models.User, claims *services.AvatarURLClaims) (*models.User, bool) {
	if claims.SubjectID == viewer.ID {
		return viewer, true
	}
	if claims.GroupID == "" {
		return nil, false
	}
	subject, err := api.users.Get(r.Context(), claims.SubjectID)
	if err != nil || subject.TenantID != viewer.TenantID {
		return nil, false
	}
	for _, userID := range []string{viewer.ID, subject.ID} {
		membership, err := api.memberships.GetByGroupAndUser(r.Context(), claims.GroupID, userID)
		if err != nil || membership.TenantID != viewer.TenantID {
			return nil, false
		}
	}
	return subject, true
}
