// Package services provides business logic services for the application.
package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

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

// SignedURLClaims contains the claims for a signed file URL
type SignedURLClaims struct {
	FileID    string    // File ID being accessed
	UserID    string    // User ID for access control
	ExpiresAt time.Time // Expiration time
}

// GenerateSignedURL creates a signed URL for file access
// The URL format will be: /files/{fileID}.{ext}?sig={signature}&exp={timestamp}&uid={userID}
func (s *FileSigningService) GenerateSignedURL(fileID, fileExt, userID string) (string, error) {
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

	// Create the message to sign: method + path + fileID + userID + expiration
	message := fmt.Sprintf("GET|%s|%s|%s|%d", basePath, fileID, userID, expTimestamp)

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
func (s *FileSigningService) GenerateSignedURLsWithThumbnails(file *models.FileEntity, userID string) (string, map[string]string, error) {
	// Get file extension (remove leading dot if present)
	fileExt := strings.TrimPrefix(file.Ext, ".")

	// Generate signed URL for the original file
	originalURL, err := s.GenerateSignedURL(file.ID, fileExt, userID)
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
			thumbnailURL, err := s.generateThumbnailSignedURL(file.ID, sizeName, userID)
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
func (s *FileSigningService) generateThumbnailSignedURL(fileID, sizeName, userID string) (string, error) {
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

	// Create the message to sign: method + path + fileID + userID + expiration
	message := fmt.Sprintf("GET|%s|%s|%s|%d", basePath, fileID, userID, expTimestamp)

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

// ValidateSignedURL validates a signed URL and returns the claims if valid
func (s *FileSigningService) ValidateSignedURL(path string, queryParams url.Values) (*SignedURLClaims, error) {
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
	message := fmt.Sprintf("GET|%s|%s|%s|%d", path, fileID, userID, expTimestamp)

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
