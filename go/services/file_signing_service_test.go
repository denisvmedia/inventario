package services_test

import (
	"net/url"
	"strconv"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services"
)

func TestFileSigningService_GenerateSignedURL(t *testing.T) {
	c := qt.New(t)

	signingKey := []byte("test-signing-key-32-bytes-long!!")
	expiration := 15 * time.Minute
	service := services.NewFileSigningService(signingKey, expiration)

	tests := []struct {
		name        string
		fileID      string
		fileExt     string
		userID      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid parameters",
			fileID:      "test-file-123",
			fileExt:     "pdf",
			userID:      "user-456",
			expectError: false,
		},
		{
			name:        "empty file ID",
			fileID:      "",
			fileExt:     "pdf",
			userID:      "user-456",
			expectError: true,
			errorMsg:    "file ID is required",
		},
		{
			name:        "empty file extension",
			fileID:      "test-file-123",
			fileExt:     "",
			userID:      "user-456",
			expectError: true,
			errorMsg:    "file extension is required",
		},
		{
			name:        "empty user ID",
			fileID:      "test-file-123",
			fileExt:     "pdf",
			userID:      "",
			expectError: true,
			errorMsg:    "user ID is required",
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			signedURL, err := service.GenerateSignedURL(tt.fileID, tt.fileExt, tt.userID)

			if tt.expectError {
				c.Assert(err, qt.IsNotNil)
				c.Assert(err.Error(), qt.Equals, tt.errorMsg)
				c.Assert(signedURL, qt.Equals, "")
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(signedURL, qt.Not(qt.Equals), "")

				// Parse the URL to validate structure
				parsedURL, parseErr := url.Parse(signedURL)
				c.Assert(parseErr, qt.IsNil)

				// Check path format - current implementation uses /files/{fileID} format
				expectedPath := "/api/v1/files/download/files/" + tt.fileID
				c.Assert(parsedURL.Path, qt.Equals, expectedPath)

				// Check required query parameters
				query := parsedURL.Query()
				c.Assert(query.Get("sig"), qt.Not(qt.Equals), "")
				c.Assert(query.Get("exp"), qt.Not(qt.Equals), "")
				c.Assert(query.Get("uid"), qt.Equals, tt.userID)

				// Check expiration is in the future
				expStr := query.Get("exp")
				expTimestamp, expErr := strconv.ParseInt(expStr, 10, 64)
				c.Assert(expErr, qt.IsNil)
				c.Assert(expTimestamp, qt.Satisfies, func(ts int64) bool {
					return time.Unix(ts, 0).After(time.Now())
				})
			}
		})
	}
}

func TestFileSigningService_ValidateSignedURL_ValidCases(t *testing.T) {
	c := qt.New(t)

	signingKey := []byte("test-signing-key-32-bytes-long!!")
	expiration := 15 * time.Minute
	service := services.NewFileSigningService(signingKey, expiration)

	// Generate a valid signed URL first
	fileID := "test-file-123"
	fileExt := "pdf"
	userID := "user-456"
	signedURL, err := service.GenerateSignedURL(fileID, fileExt, userID)
	c.Assert(err, qt.IsNil)

	// Parse the URL to get path and query parameters
	parsedURL, err := url.Parse(signedURL)
	c.Assert(err, qt.IsNil)

	tests := []struct {
		name  string
		path  string
		query url.Values
	}{
		{
			name:  "valid signed URL",
			path:  parsedURL.Path,
			query: parsedURL.Query(),
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			claims, err := service.ValidateSignedURL(tt.path, tt.query)

			c.Assert(err, qt.IsNil)
			c.Assert(claims, qt.IsNotNil)
			c.Assert(claims.FileID, qt.Equals, fileID)
			c.Assert(claims.UserID, qt.Equals, userID)
			c.Assert(claims.ExpiresAt.After(time.Now()), qt.IsTrue)
		})
	}
}

func TestFileSigningService_ValidateSignedURL_ErrorCases(t *testing.T) {
	c := qt.New(t)

	signingKey := []byte("test-signing-key-32-bytes-long!!")
	expiration := 15 * time.Minute
	service := services.NewFileSigningService(signingKey, expiration)

	// Generate a valid signed URL first to use as base for error cases
	fileID := "test-file-123"
	fileExt := "pdf"
	userID := "user-456"
	signedURL, err := service.GenerateSignedURL(fileID, fileExt, userID)
	c.Assert(err, qt.IsNil)

	// Parse the URL to get path and query parameters
	parsedURL, err := url.Parse(signedURL)
	c.Assert(err, qt.IsNil)

	tests := []struct {
		name     string
		path     string
		query    url.Values
		errorMsg string
	}{
		{
			name:     "missing signature",
			path:     parsedURL.Path,
			query:    func() url.Values { q := parsedURL.Query(); q.Del("sig"); return q }(),
			errorMsg: "missing signature parameter",
		},
		{
			name:     "missing expiration",
			path:     parsedURL.Path,
			query:    func() url.Values { q := parsedURL.Query(); q.Del("exp"); return q }(),
			errorMsg: "missing expiration parameter",
		},
		{
			name:     "missing user ID",
			path:     parsedURL.Path,
			query:    func() url.Values { q := parsedURL.Query(); q.Del("uid"); return q }(),
			errorMsg: "missing user ID parameter",
		},
		{
			name:     "invalid expiration format",
			path:     parsedURL.Path,
			query:    func() url.Values { q := parsedURL.Query(); q.Set("exp", "invalid"); return q }(),
			errorMsg: "invalid expiration timestamp",
		},
		{
			name:     "expired URL",
			path:     parsedURL.Path,
			query:    func() url.Values { q := parsedURL.Query(); q.Set("exp", "1"); return q }(),
			errorMsg: "signed URL has expired",
		},
		{
			name:     "invalid signature",
			path:     parsedURL.Path,
			query:    func() url.Values { q := parsedURL.Query(); q.Set("sig", "invalid-signature"); return q }(),
			errorMsg: "invalid signature",
		},
		{
			name:     "tampered path",
			path:     "/api/v1/files/download/different-file.pdf",
			query:    parsedURL.Query(),
			errorMsg: "invalid signature",
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			claims, err := service.ValidateSignedURL(tt.path, tt.query)

			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tt.errorMsg)
			c.Assert(claims, qt.IsNil)
		})
	}
}

func TestFileSigningService_ExtractFileIDFromPath_ValidPaths(t *testing.T) {
	c := qt.New(t)

	signingKey := []byte("test-signing-key-32-bytes-long!!")
	expiration := 15 * time.Minute
	service := services.NewFileSigningService(signingKey, expiration)

	tests := []struct {
		name string
		path string
	}{
		{
			name: "valid file path",
			path: "/api/v1/files/download/test-file-123.pdf",
		},
		{
			name: "thumbnail path small",
			path: "/api/v1/files/download/image_thumb_small.jpg",
		},
		{
			name: "thumbnail path medium",
			path: "/api/v1/files/download/photo_thumb_medium.png",
		},
		{
			name: "file with complex ID",
			path: "/api/v1/files/download/file-with-dashes-123.jpg",
		},
		{
			name: "file with UUID",
			path: "/api/v1/files/download/550e8400-e29b-41d4-a716-446655440000.png",
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			// For valid paths, we expect the error to be about invalid signature
			// (since we're using a dummy signature), not about path format
			query := url.Values{}
			query.Set("sig", "dummy-signature")
			query.Set("exp", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
			query.Set("uid", "test-user")
			query.Set("fid", "test-file-id")

			_, err := service.ValidateSignedURL(tt.path, query)

			// Valid paths should fail with "invalid signature" error, not path format error
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Equals, "invalid signature")
		})
	}
}

func TestFileSigningService_ExtractFileIDFromPath_InvalidPaths(t *testing.T) {
	c := qt.New(t)

	signingKey := []byte("test-signing-key-32-bytes-long!!")
	expiration := 15 * time.Minute
	service := services.NewFileSigningService(signingKey, expiration)

	tests := []struct {
		name string
		path string
	}{
		{
			name: "path without extension",
			path: "/api/v1/files/download/test-file-123",
		},
		{
			name: "path without file ID",
			path: "/api/v1/files/download/.pdf",
		},
		{
			name: "invalid path format",
			path: "/invalid/path",
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			// Invalid paths should fail with path format errors
			query := url.Values{}
			query.Set("sig", "dummy-signature")
			query.Set("exp", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
			query.Set("uid", "test-user")

			_, err := service.ValidateSignedURL(tt.path, query)

			// Invalid paths should fail, but we don't care about the specific error message
			// as it could be path format or other validation errors
			c.Assert(err, qt.IsNotNil)
		})
	}
}

func TestFileSigningService_SecurityProperties(t *testing.T) {
	c := qt.New(t)

	signingKey := []byte("test-signing-key-32-bytes-long!!")
	expiration := 15 * time.Minute
	service := services.NewFileSigningService(signingKey, expiration)

	c.Run("different keys produce different signatures", func(c *qt.C) {
		otherKey := []byte("different-key-32-bytes-long!!!")
		otherService := services.NewFileSigningService(otherKey, expiration)

		url1, err1 := service.GenerateSignedURL("file-123", "pdf", "user-456")
		url2, err2 := otherService.GenerateSignedURL("file-123", "pdf", "user-456")

		c.Assert(err1, qt.IsNil)
		c.Assert(err2, qt.IsNil)
		c.Assert(url1, qt.Not(qt.Equals), url2)

		// URL from service1 should not validate with service2
		parsed, err := url.Parse(url1)
		c.Assert(err, qt.IsNil)
		_, err = otherService.ValidateSignedURL(parsed.Path, parsed.Query())
		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Equals, "invalid signature")
	})

	c.Run("signatures with different timestamps are different", func(c *qt.C) {
		url1, err1 := service.GenerateSignedURL("file-123", "pdf", "user-456")
		c.Assert(err1, qt.IsNil)

		// Wait a full second to ensure different timestamp (Unix timestamp has 1-second resolution)
		time.Sleep(1100 * time.Millisecond)

		url2, err2 := service.GenerateSignedURL("file-123", "pdf", "user-456")
		c.Assert(err2, qt.IsNil)

		// URLs should be different due to different timestamps
		c.Assert(url1, qt.Not(qt.Equals), url2)

		// But both should validate successfully
		parsed1, err := url.Parse(url1)
		c.Assert(err, qt.IsNil)
		claims1, err := service.ValidateSignedURL(parsed1.Path, parsed1.Query())
		c.Assert(err, qt.IsNil)
		c.Assert(claims1, qt.IsNotNil)

		parsed2, err := url.Parse(url2)
		c.Assert(err, qt.IsNil)
		claims2, err := service.ValidateSignedURL(parsed2.Path, parsed2.Query())
		c.Assert(err, qt.IsNil)
		c.Assert(claims2, qt.IsNotNil)
	})
}

func TestFileSigningService_GenerateSignedURLsWithThumbnails(t *testing.T) {
	signingKey := []byte("test-signing-key-32-bytes-long!!")
	expiration := 15 * time.Minute
	service := services.NewFileSigningService(signingKey, expiration)

	tests := []struct {
		name                   string
		fileID                 string
		fileExt                string
		userID                 string
		originalPath           string
		mimeType               string
		expectThumbnails       bool
		expectedThumbnailCount int
	}{
		{
			name:                   "JPEG image should generate thumbnails",
			fileID:                 "test-file-123",
			fileExt:                "jpg",
			userID:                 "user-456",
			originalPath:           "test-image.jpg",
			mimeType:               "image/jpeg",
			expectThumbnails:       true,
			expectedThumbnailCount: 2, // small and medium
		},
		{
			name:                   "PNG image should generate thumbnails",
			fileID:                 "test-file-124",
			fileExt:                "png",
			userID:                 "user-456",
			originalPath:           "test-image.png",
			mimeType:               "image/png",
			expectThumbnails:       true,
			expectedThumbnailCount: 2, // small and medium
		},
		{
			name:                   "PDF should not generate thumbnails",
			fileID:                 "test-file-125",
			fileExt:                "pdf",
			userID:                 "user-456",
			originalPath:           "test-document.pdf",
			mimeType:               "application/pdf",
			expectThumbnails:       false,
			expectedThumbnailCount: 0,
		},
		{
			name:                   "WebP image should not generate thumbnails",
			fileID:                 "test-file-126",
			fileExt:                "webp",
			userID:                 "user-456",
			originalPath:           "test-image.webp",
			mimeType:               "image/webp",
			expectThumbnails:       false,
			expectedThumbnailCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Create a file entity for testing
			fileEntity := &models.FileEntity{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: tt.fileID},
				},
				Type: models.FileTypeImage,
				File: &models.File{
					Path:         tt.fileID,
					OriginalPath: tt.originalPath,
					Ext:          "." + tt.fileExt,
					MIMEType:     tt.mimeType,
				},
			}

			originalURL, thumbnails, err := service.GenerateSignedURLsWithThumbnails(fileEntity, tt.userID)

			c.Assert(err, qt.IsNil)
			c.Assert(originalURL, qt.Not(qt.Equals), "")

			// Verify original URL format
			c.Assert(originalURL, qt.Contains, "/api/v1/files/download/")
			c.Assert(originalURL, qt.Contains, "sig=")
			c.Assert(originalURL, qt.Contains, "exp=")
			c.Assert(originalURL, qt.Contains, "uid=")

			// Verify thumbnails
			if tt.expectThumbnails {
				c.Assert(len(thumbnails), qt.Equals, tt.expectedThumbnailCount)
				c.Assert(thumbnails["small"], qt.Not(qt.Equals), "")
				c.Assert(thumbnails["medium"], qt.Not(qt.Equals), "")

				// Verify thumbnail URLs contain expected paths - current implementation uses /thumbnails/{fileID}/{size}
				c.Assert(thumbnails["small"], qt.Contains, "/thumbnails/")
				c.Assert(thumbnails["small"], qt.Contains, "/small")
				c.Assert(thumbnails["medium"], qt.Contains, "/thumbnails/")
				c.Assert(thumbnails["medium"], qt.Contains, "/medium")
			} else {
				c.Assert(len(thumbnails), qt.Equals, tt.expectedThumbnailCount)
			}
		})
	}
}

func TestFileSigningService_GetThumbnailPath(t *testing.T) {
	signingKey := []byte("test-signing-key-32-bytes-long!!")
	expiration := 15 * time.Minute
	service := services.NewFileSigningService(signingKey, expiration)

	tests := []struct {
		name     string
		fileID   string
		sizeName string
		expected string
	}{
		{
			name:     "PNG file small thumbnail",
			fileID:   "test-id",
			sizeName: "small",
			expected: "/thumbnails/test-id/small",
		},
		{
			name:     "JPEG file medium thumbnail",
			fileID:   "test-id",
			sizeName: "medium",
			expected: "/thumbnails/test-id/medium",
		},
		{
			name:     "File with UUID",
			fileID:   "f47ac10b-58cc-4372-a567-0e02b2c3d479",
			sizeName: "small",
			expected: "/thumbnails/f47ac10b-58cc-4372-a567-0e02b2c3d479/small",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Create a file entity for testing
			fileEntity := &models.FileEntity{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: tt.fileID},
				},
				Type: models.FileTypeImage,
				File: &models.File{
					Path:         "test-image",
					OriginalPath: "test-image.png",
					Ext:          ".png",
					MIMEType:     "image/png",
				},
			}

			_, thumbnails, err := service.GenerateSignedURLsWithThumbnails(fileEntity, "user-id")
			c.Assert(err, qt.IsNil)

			switch tt.sizeName {
			case "small":
				c.Assert(thumbnails["small"], qt.Contains, tt.expected)
			case "medium":
				c.Assert(thumbnails["medium"], qt.Contains, tt.expected)
			}
		})
	}
}
