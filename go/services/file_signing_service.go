// Package services provides business logic services for the application.
package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/mimekit"
	"github.com/denisvmedia/inventario/models"
)

// FileSigningService provides secure file URL signing functionality
type FileSigningService struct {
	signingKey []byte
	expiration time.Duration
}

// NewFileSigningService creates a new file signing service
func NewFileSigningService(signingKey []byte, expiration time.Duration) *FileSigningService {
	return &FileSigningService{
		signingKey: signingKey,
		expiration: expiration,
	}
}

// SessionBinding is the per-session identifier woven into the signed URL.
// It is derived from the refresh_token cookie (see ExtractSessionBinding)
// and folded into the HMAC message; it is NOT carried as a query parameter
// so it cannot leak via Referer / proxy logs / browser history alongside
// the URL itself.
type SessionBinding string

// ExtractSessionBinding derives the binding from the request's refresh_token
// cookie. Returns "" when the cookie is missing or empty — the URL is then
// unbound (backwards-compatible for non-browser clients without cookies).
//
// The binding is the first 16 bytes of SHA-256(cookie value), base64url-
// encoded. The relevant security property is preimage resistance over a
// high-entropy cookie value (a real refresh token is 32 random bytes), so
// 16 bytes is overkill — it also keeps the binding identifier itself
// short, which matters for log volume and `fmt.Sprintf` of the HMAC
// message but not for the URL (binding is never written into the URL).
// The HMAC still supplies the full 32 bytes of secret entropy.
//
// Impersonation caveat: during an impersonation session the cookie value
// is `imp:<jti>`, where the JTI is also a claim on the visible access
// token. The binding therefore degrades to "knowledge of the access
// token" for impersonation; a separate leak of both the URL and the
// bearer is required to replay. See #1781 follow-up for tighter binding.
func ExtractSessionBinding(r *http.Request) SessionBinding {
	if r == nil {
		return ""
	}
	cookie, err := r.Cookie(appctx.RefreshTokenCookieName)
	if err != nil || cookie == nil || cookie.Value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(cookie.Value))
	return SessionBinding(base64.RawURLEncoding.EncodeToString(sum[:16]))
}

// SignedURLClaims contains the claims for a signed file URL
type SignedURLClaims struct {
	FileID    string    // File ID being accessed
	UserID    string    // User ID for access control
	ExpiresAt time.Time // Expiration time
}

// Disposition selects how a signed file URL serves its bytes: as an
// attachment download (the default) or inline for in-browser viewing
// (the "Open in new tab" affordance, #1962). The choice is folded into
// the HMAC message — and surfaced as a `disposition=inline` query
// parameter only for the inline case — so it cannot be tampered with
// after the URL is minted: flipping an attachment URL to inline (or
// stripping inline off an inline URL) changes the validated message and
// fails the signature check.
type Disposition int

const (
	// DispositionAttachment serves the file as a download. The URL and
	// signed message are byte-identical to the pre-#1962 format, so
	// existing attachment URLs are unaffected.
	DispositionAttachment Disposition = iota
	// DispositionInline serves the file inline for viewing. Adds the
	// `|inline` suffix to the signed message and a `disposition=inline`
	// query parameter to the URL.
	DispositionInline
)

// dispositionInlineParam is the literal query value (and HMAC suffix
// token) that marks an inline URL. Kept as one constant so the generate
// and validate sides cannot drift.
const dispositionInlineParam = "inline"

// GenerateSignedURL creates a signed URL for file access.
//
// The emitted URL is `/api/v1/files/download/files/{fileID}?sig=…&exp=…&uid=…&fid=…`
// — no extension is embedded; chi routes off the path segment alone and
// the streamer reads MIME / extension from the FileEntity. `fileExt` is
// validated for non-emptiness as a sanity check but does not appear in
// the path.
//
// `binding` is the SessionBinding from the request that authorized this
// signing call. An empty binding produces an unbound URL — validators
// must be invoked with an empty binding to accept it. The binding is
// folded into the HMAC message only and never written into the URL.
//
// This is the attachment (download) variant; GenerateInlineSignedURL
// mints the inline (in-browser viewing) variant.
func (s *FileSigningService) GenerateSignedURL(fileID, fileExt, userID string, binding SessionBinding) (string, error) {
	return s.generateSignedURL(fileID, fileExt, userID, binding, DispositionAttachment)
}

// GenerateInlineSignedURL mints a signed URL that serves the file inline
// (Content-Disposition: inline) for in-browser viewing — the frontend's
// "Open in new tab" action (#1962). The serve handler still only honours
// inline for content types that are safe to render same-origin (see
// mimekit.IsInlineSafe); for everything else it falls back to an
// attachment download, so this URL is never dangerous even for an
// HTML/SVG upload.
func (s *FileSigningService) GenerateInlineSignedURL(fileID, fileExt, userID string, binding SessionBinding) (string, error) {
	return s.generateSignedURL(fileID, fileExt, userID, binding, DispositionInline)
}

func (s *FileSigningService) generateSignedURL(fileID, fileExt, userID string, binding SessionBinding, disposition Disposition) (string, error) {
	if fileID == "" {
		return "", errors.New("file ID is required")
	}
	if fileExt == "" {
		return "", errors.New("file extension is required")
	}
	if userID == "" {
		return "", errors.New("user ID is required")
	}

	// Calculate expiration time
	expiresAt := time.Now().Add(s.expiration)
	expTimestamp := expiresAt.Unix()

	// Create the base URL path for original files
	basePath := fmt.Sprintf("/api/v1/files/download/files/%s", fileID)

	// Create the message to sign: method + path + fileID + userID + expiration + binding.
	// The |binding segment is unconditional — pre-#1781 URLs are not
	// supported on this code path; in-flight unbound URLs are expected
	// to 401 after a deploy and be re-fetched by the FE.
	message := fmt.Sprintf("GET|%s|%s|%s|%d|%s", basePath, fileID, userID, expTimestamp, binding)
	// Inline serving folds a `|inline` suffix into the signed message so
	// the disposition is tamper-proof. Attachment URLs keep the exact
	// pre-#1962 message (no suffix), so existing URLs are unaffected.
	if disposition == DispositionInline {
		message += "|" + dispositionInlineParam
	}

	// Generate HMAC signature
	signature, err := s.generateSignature(message)
	if err != nil {
		return "", errxtrace.Wrap("failed to generate signature", err)
	}

	// Build the signed URL with query parameters, including file ID for validation
	signedURL := fmt.Sprintf("%s?sig=%s&exp=%d&uid=%s&fid=%s",
		basePath,
		url.QueryEscape(signature),
		expTimestamp,
		url.QueryEscape(userID),
		url.QueryEscape(fileID))
	if disposition == DispositionInline {
		signedURL += "&disposition=" + dispositionInlineParam
	}

	return signedURL, nil
}

// GenerateSignedURLsWithThumbnails mints a signed URL for the original
// file and best-effort signed URLs for any pre-computed thumbnails the
// FE may want to render alongside it.
//
// Parameters:
//   - file: the FileEntity being exposed. `file.ID` is the routing key,
//     `file.Ext` provides the sanity-check extension for the original
//     URL, and `file.MIMEType` decides whether thumbnails are minted
//     at all (only image/jpeg and image/png are eligible — every other
//     MIME type returns an empty `thumbnails` map).
//   - userID: the user the URLs are minted for; folded into the HMAC so
//     a foreign-user replay fails validation.
//   - binding: the SessionBinding from the request that authorized this
//     signing call (see ExtractSessionBinding). Pass "" to produce
//     unbound URLs; otherwise downstream validators must present the
//     same binding to consume them.
//
// Return values:
//   - original: the signed URL for the full-resolution file. Always
//     populated when err is nil.
//   - thumbnails: keyed by size name ("small", "medium"). Only carries
//     entries for sizes that could be signed; a missing entry simply
//     means no thumbnail is available at that size and callers should
//     fall back to `original`. Non-nil but empty when the file has no
//     eligible MIME type.
//   - err: non-nil only when the *original* URL cannot be signed.
//     Per-thumbnail signing errors are swallowed by design — a missing
//     thumbnail URL is recoverable on the client; an unsigned original
//     is not.
func (s *FileSigningService) GenerateSignedURLsWithThumbnails(
	file *models.FileEntity,
	userID string,
	binding SessionBinding,
) (original string, thumbnails map[string]string, err error) {
	// Get file extension (remove leading dot if present)
	fileExt := strings.TrimPrefix(file.Ext, ".")

	// Generate signed URL for the original file
	original, err = s.GenerateSignedURL(file.ID, fileExt, userID, binding)
	if err != nil {
		return "", nil, errxtrace.Wrap("failed to generate original file URL", err)
	}

	// Generate thumbnail URLs if it's a supported image format
	thumbnails = make(map[string]string)
	if mimekit.IsImage(file.MIMEType) && (strings.HasPrefix(file.MIMEType, "image/jpeg") || strings.HasPrefix(file.MIMEType, "image/png")) {
		thumbnailSizes := map[string]int{
			"small":  150,
			"medium": 300,
		}

		for sizeName := range thumbnailSizes {
			thumbnailURL, thumbErr := s.generateThumbnailSignedURL(file.ID, sizeName, userID, binding)
			if thumbErr != nil {
				// Don't fail if thumbnail URL generation fails — thumbnail
				// may not exist yet (deferred generation) or be unsupported
				// at this size. The client falls back to `original`.
				continue
			}
			thumbnails[sizeName] = thumbnailURL
		}
	}

	return original, thumbnails, nil
}

// generateThumbnailSignedURL creates a signed URL for thumbnail access
func (s *FileSigningService) generateThumbnailSignedURL(fileID, sizeName, userID string, binding SessionBinding) (string, error) {
	if fileID == "" {
		return "", errors.New("file ID is required")
	}
	if sizeName == "" {
		return "", errors.New("size name is required")
	}
	if userID == "" {
		return "", errors.New("user ID is required")
	}

	// Calculate expiration time
	expiresAt := time.Now().Add(s.expiration)
	expTimestamp := expiresAt.Unix()

	// Create the base URL path for thumbnails
	basePath := fmt.Sprintf("/api/v1/files/download/thumbnails/%s/%s", fileID, sizeName)

	// Create the message to sign: method + path + fileID + userID + expiration + binding
	message := fmt.Sprintf("GET|%s|%s|%s|%d|%s", basePath, fileID, userID, expTimestamp, binding)

	// Generate HMAC signature
	signature, err := s.generateSignature(message)
	if err != nil {
		return "", errxtrace.Wrap("failed to generate signature", err)
	}

	// Build the signed URL with query parameters
	signedURL := fmt.Sprintf("%s?sig=%s&exp=%d&uid=%s&fid=%s",
		basePath,
		url.QueryEscape(signature),
		expTimestamp,
		url.QueryEscape(userID),
		url.QueryEscape(fileID))

	return signedURL, nil
}

// ValidateSignedURL validates a signed URL and returns the claims if valid.
//
// `binding` is derived from the validating request's refresh_token cookie
// (see ExtractSessionBinding). The signature only matches when the binding
// at validate time equals the binding the URL was minted with — that is
// what couples a URL to the session that produced it.
func (s *FileSigningService) ValidateSignedURL(path string, queryParams url.Values, binding SessionBinding) (*SignedURLClaims, error) {
	// Extract required parameters
	signature := queryParams.Get("sig")
	if signature == "" {
		return nil, errors.New("missing signature parameter")
	}

	expStr := queryParams.Get("exp")
	if expStr == "" {
		return nil, errors.New("missing expiration parameter")
	}

	userID := queryParams.Get("uid")
	if userID == "" {
		return nil, errors.New("missing user ID parameter")
	}

	fileID := queryParams.Get("fid")
	if fileID == "" {
		return nil, errors.New("missing file ID parameter")
	}

	// Parse expiration timestamp
	expTimestamp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return nil, errxtrace.Wrap("invalid expiration timestamp", err)
	}

	expiresAt := time.Unix(expTimestamp, 0)

	// Check if the URL has expired
	if time.Now().After(expiresAt) {
		return nil, errors.New("signed URL has expired")
	}

	// Recreate the message that was signed
	message := fmt.Sprintf("GET|%s|%s|%s|%d|%s", path, fileID, userID, expTimestamp, binding)
	// Mirror the generate side: only an explicit `disposition=inline`
	// folds the `|inline` suffix into the message. Any other value (or
	// none) reproduces the attachment message. This makes the
	// disposition tamper-evident — appending `disposition=inline` to an
	// attachment URL changes the validated message and fails the
	// signature check, so inline serving can't be forced onto a URL that
	// wasn't signed for it.
	if queryParams.Get("disposition") == dispositionInlineParam {
		message += "|" + dispositionInlineParam
	}

	// Validate the signature
	if !s.validateSignature(message, signature) {
		return nil, errors.New("invalid signature")
	}

	// Return the validated claims
	return &SignedURLClaims{
		FileID:    fileID,
		UserID:    userID,
		ExpiresAt: expiresAt,
	}, nil
}

// generateSignature creates an HMAC-SHA256 signature for the given message
func (s *FileSigningService) generateSignature(message string) (string, error) {
	h := hmac.New(sha256.New, s.signingKey)
	_, err := h.Write([]byte(message))
	if err != nil {
		return "", errxtrace.Wrap("failed to write message to HMAC", err)
	}

	signature := base64.URLEncoding.EncodeToString(h.Sum(nil))
	return signature, nil
}

// validateSignature validates an HMAC-SHA256 signature against the given message
func (s *FileSigningService) validateSignature(message, signature string) bool {
	expectedSignature, err := s.generateSignature(message)
	if err != nil {
		return false
	}

	// Use constant-time comparison to prevent timing attacks
	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}
