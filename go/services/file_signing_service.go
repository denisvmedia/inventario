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
	"time"

	"github.com/denisvmedia/inventario/internal/errkit"
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

	// Create the base URL path
	basePath := fmt.Sprintf("/api/v1/files/download/%s.%s", fileID, fileExt)

	// Create the message to sign: method + path + fileID + userID + expiration
	message := fmt.Sprintf("GET|%s|%s|%s|%d", basePath, fileID, userID, expTimestamp)

	// Generate HMAC signature
	signature, err := s.generateSignature(message)
	if err != nil {
		return "", errkit.Wrap(err, "failed to generate signature")
	}

	// Build the signed URL with query parameters
	signedURL := fmt.Sprintf("%s?sig=%s&exp=%d&uid=%s",
		basePath,
		url.QueryEscape(signature),
		expTimestamp,
		url.QueryEscape(userID))

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

	// Extract file ID from path (format: /api/v1/files/{fileID}.{ext})
	fileID, err := s.extractFileIDFromPath(path)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to extract file ID from path")
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
// Expected format: /api/v1/files/download/{fileID}.{ext}
func (s *FileSigningService) extractFileIDFromPath(path string) (string, error) {
	// Parse the URL path to extract file ID
	// Example: /api/v1/files/download/123.pdf -> fileID = "123"

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

	fileID := path[lastSlash+1 : lastDot]
	if fileID == "" {
		return "", errors.New("empty file ID in path")
	}

	return fileID, nil
}
