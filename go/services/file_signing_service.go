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

// GenerateSignedURL creates a signed URL for file access
// The URL format will be: /files/{fileID}.{ext}?sig={signature}&exp={timestamp}&uid={userID}
//
// `binding` is the SessionBinding from the request that authorized this
// signing call. An empty binding produces an unbound URL — validators must
// be invoked with an empty binding to accept it.
func (s *FileSigningService) GenerateSignedURL(fileID, fileExt, userID string, binding SessionBinding) (string, error) {
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

	// Create the message to sign: method + path + fileID + userID + expiration + binding
	message := fmt.Sprintf("GET|%s|%s|%s|%d|%s", basePath, fileID, userID, expTimestamp, binding)

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

	return signedURL, nil
}

// GenerateSignedURLsWithThumbnails generates signed URLs for a file and its thumbnails
func (s *FileSigningService) GenerateSignedURLsWithThumbnails(file *models.FileEntity, userID string, binding SessionBinding) (string, map[string]string, error) {
	// Get file extension (remove leading dot if present)
	fileExt := strings.TrimPrefix(file.Ext, ".")

	// Generate signed URL for the original file
	originalURL, err := s.GenerateSignedURL(file.ID, fileExt, userID, binding)
	if err != nil {
		return "", nil, errxtrace.Wrap("failed to generate original file URL", err)
	}

	// Generate thumbnail URLs if it's a supported image format
	thumbnails := make(map[string]string)
	if mimekit.IsImage(file.MIMEType) && (strings.HasPrefix(file.MIMEType, "image/jpeg") || strings.HasPrefix(file.MIMEType, "image/png")) {
		thumbnailSizes := map[string]int{
			"small":  150,
			"medium": 300,
		}

		for sizeName := range thumbnailSizes {
			thumbnailURL, err := s.generateThumbnailSignedURL(file.ID, sizeName, userID, binding)
			if err != nil {
				// Don't fail if thumbnail URL generation fails - thumbnail might not exist
				continue
			}
			thumbnails[sizeName] = thumbnailURL
		}
	}

	return originalURL, thumbnails, nil
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
