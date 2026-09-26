package apiserver

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// avatarsAPI serves the profile photo endpoints (#1382).
//
// Reads are mounted twice: once on /auth/me for the owner and once inside the
// group subtree for everybody else. That is what decides who may see a photo —
// the group routes already require membership, so the rule is "you can see the
// avatar of someone you share a group with", which is the same rule that lets
// you see their name at all. There is deliberately no route that takes a bare
// user id, because it would have no such rule behind it.
type avatarsAPI struct {
	avatarService *services.AvatarService
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
// @Description Accepts JPEG or PNG up to 2 MB, detected from the content rather than from the declared type or the filename.
// @Tags auth
// @Accept multipart/form-data
// @Produce json
// @Param avatar formData file true "Image file"
// @Success 200 {object} map[string]string "The stored avatar path"
// @Failure 400 {string} string "Not an image, or larger than 2 MB"
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
	if err := r.ParseMultipartForm(avatarMultipartMemory); err != nil { //nolint:gosec // G120: the body is bounded by the MaxBytesReader above
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
	case err != nil:
		_ = internalServerError(w, r, err)
		return
	}

	writeAvatarPath(w, r, path)
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
// @Param groupID path string true "Group ID or slug"
// @Param memberUserID path string true "Member user ID"
// @Success 200 {file} file "The avatar image"
// @Failure 403 {string} string "Not a member of this group"
// @Failure 404 {string} string "No avatar set, or not a member of this group"
// @Router /groups/{groupID}/members/{memberUserID}/avatar [get].
func (api *avatarsAPI) handleGetMemberAvatar(w http.ResponseWriter, r *http.Request) {
	api.streamAvatar(w, r, chi.URLParam(r, "memberUserID"))
}

// streamAvatar writes the stored image, or 404 when there is none.
func (api *avatarsAPI) streamAvatar(w http.ResponseWriter, r *http.Request, userID string) {
	reader, contentType, err := api.avatarService.Open(r.Context(), userID)
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

// writeAvatarPath answers an upload with the stored key, which is what the
// frontend swaps into its cache-busting image URL.
func writeAvatarPath(w http.ResponseWriter, r *http.Request, path string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	body := `{"avatar_path":` + strconv.Quote(path) + `}`
	if _, err := w.Write([]byte(body)); err != nil {
		_ = r // nothing to do: the response is already committed
	}
}
