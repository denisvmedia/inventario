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

	"github.com/denisvmedia/inventario/internal/errkit"
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
		return "", errkit.Wrap(err, "failed to generate signature")
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
		return "", nil, errkit.Wrap(err, "failed to generate original file URL")
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
		return "", errkit.Wrap(err, "failed to generate signature")
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

// getThumbnailPath generates the thumbnail file path using file ID
// All thumbnails are saved as JPEG files regardless of the original format
func (s *FileSigningService) getThumbnailPath(fileID, sizeName string) string {
	// Use file ID for thumbnail paths to avoid conflicts with user-controlled paths
	return fmt.Sprintf("thumbnails/%s_%s.jpg", fileID, sizeName)
}

// generateSignedURLForPath generates a signed URL for a specific file path
func (s *FileSigningService) generateSignedURLForPath(fileID, fileExt, userID, filePath string) (string, error) {
	// Calculate expiration time
	expiresAt := time.Now().Add(s.expiration)
	expTimestamp := expiresAt.Unix()

	// Create the base URL path using the file path
	basePath := fmt.Sprintf("/api/v1/files/download/%s", filePath)

	// Create the message to sign: method + path + fileID + userID + expiration
	message := fmt.Sprintf("GET|%s|%s|%s|%d", basePath, fileID, userID, expTimestamp)

	// Generate HMAC signature
	signature, err := s.generateSignature(message)
	if err != nil {
		return "", errkit.Wrap(err, "failed to generate signature")
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
		return nil, errkit.Wrap(err, "invalid expiration timestamp")
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
		return "", errkit.Wrap(err, "failed to write message to HMAC")
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

// extractFileIDFromPath extracts the file ID from a file path
// Expected formats:
//   - /api/v1/files/download/{fileID}.{ext}
//   - /api/v1/files/download/{filename}_thumb_{size}.{ext} (for thumbnails)
func (s *FileSigningService) extractFileIDFromPath(path string) (string, error) {
	// Parse the URL path to extract file ID
	// Example: /api/v1/files/download/123.pdf -> fileID = "123"
	// Example: /api/v1/files/download/image_thumb_medium.jpg -> fileID = "image" (original filename)

	// Find the last slash and the last dot
	lastSlash := -1
	lastDot := -1

	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' && lastDot == -1 {
			lastDot = i
		}
		if path[i] == '/' && lastSlash == -1 {
			lastSlash = i
			break
		}
	}

	if lastSlash == -1 || lastDot == -1 || lastSlash >= lastDot {
		return "", errors.New("invalid file path format")
	}

	filename := path[lastSlash+1 : lastDot]
	if filename == "" {
		return "", errors.New("empty filename in path")
	}

	// Check if this is a thumbnail path (contains "_thumb_")
	if strings.Contains(filename, "_thumb_") {
		// Extract the original filename before "_thumb_"
		parts := strings.Split(filename, "_thumb_")
		if len(parts) >= 2 && parts[0] != "" {
			return parts[0], nil
		}
		return "", errors.New("invalid thumbnail path format")
	}

	// Regular file path - return the filename as file ID
	return filename, nil
}
