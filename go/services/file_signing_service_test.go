package services_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services"
)

// newTestRefreshCookie builds a refresh_token cookie suitable for binding
// tests. Production cookies are emitted by apiserver.writeRefreshCookie with
// the same attributes; setting them here keeps the test fixture aligned and
// silences gosec G124 without sprinkling //#nosec directives. The transport
// security flags do not affect ExtractSessionBinding's behaviour — the
// helper only reads Name + Value.
func newTestRefreshCookie(value string) *http.Cookie {
	return &http.Cookie{
		Name:     appctx.RefreshTokenCookieName,
		Value:    value,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
}

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
			signedURL, err := service.GenerateSignedURL(tt.fileID, tt.fileExt, tt.userID, "")

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

				// The binding MUST NOT leak as a query parameter — it
				// is folded into the HMAC only. If a future change ever
				// surfaces it in the URL the leak this issue closes
				// (#1781) reopens immediately.
				c.Assert(query.Get("binding"), qt.Equals, "")
				c.Assert(query.Get("sb"), qt.Equals, "")

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
	signedURL, err := service.GenerateSignedURL(fileID, fileExt, userID, "")
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
			claims, err := service.ValidateSignedURL(tt.path, tt.query, "")

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
	signedURL, err := service.GenerateSignedURL(fileID, fileExt, userID, "")
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
			claims, err := service.ValidateSignedURL(tt.path, tt.query, "")

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

			_, err := service.ValidateSignedURL(tt.path, query, "")

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

			_, err := service.ValidateSignedURL(tt.path, query, "")

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

		url1, err1 := service.GenerateSignedURL("file-123", "pdf", "user-456", "")
		url2, err2 := otherService.GenerateSignedURL("file-123", "pdf", "user-456", "")

		c.Assert(err1, qt.IsNil)
		c.Assert(err2, qt.IsNil)
		c.Assert(url1, qt.Not(qt.Equals), url2)

		// URL from service1 should not validate with service2
		parsed, err := url.Parse(url1)
		c.Assert(err, qt.IsNil)
		_, err = otherService.ValidateSignedURL(parsed.Path, parsed.Query(), "")
		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Equals, "invalid signature")
	})

	c.Run("signatures with different timestamps are different", func(c *qt.C) {
		url1, err1 := service.GenerateSignedURL("file-123", "pdf", "user-456", "")
		c.Assert(err1, qt.IsNil)

		// Wait a full second to ensure different timestamp (Unix timestamp has 1-second resolution)
		time.Sleep(1100 * time.Millisecond)

		url2, err2 := service.GenerateSignedURL("file-123", "pdf", "user-456", "")
		c.Assert(err2, qt.IsNil)

		// URLs should be different due to different timestamps
		c.Assert(url1, qt.Not(qt.Equals), url2)

		// But both should validate successfully
		parsed1, err := url.Parse(url1)
		c.Assert(err, qt.IsNil)
		claims1, err := service.ValidateSignedURL(parsed1.Path, parsed1.Query(), "")
		c.Assert(err, qt.IsNil)
		c.Assert(claims1, qt.IsNotNil)

		parsed2, err := url.Parse(url2)
		c.Assert(err, qt.IsNil)
		claims2, err := service.ValidateSignedURL(parsed2.Path, parsed2.Query(), "")
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
				TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
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

			originalURL, thumbnails, err := service.GenerateSignedURLsWithThumbnails(fileEntity, tt.userID, "")

			c.Assert(err, qt.IsNil)
			c.Assert(originalURL, qt.Not(qt.Equals), "")

			// Verify original URL format
			c.Assert(originalURL, qt.Contains, "/api/v1/files/download/")
			c.Assert(originalURL, qt.Contains, "sig=")
			c.Assert(originalURL, qt.Contains, "exp=")
			c.Assert(originalURL, qt.Contains, "uid=")

			// Verify thumbnails
			if tt.expectThumbnails {
				c.Assert(thumbnails, qt.HasLen, tt.expectedThumbnailCount)
				c.Assert(thumbnails["small"], qt.Not(qt.Equals), "")
				c.Assert(thumbnails["medium"], qt.Not(qt.Equals), "")

				// Verify thumbnail URLs contain expected paths - current implementation uses /thumbnails/{fileID}/{size}
				c.Assert(thumbnails["small"], qt.Contains, "/thumbnails/")
				c.Assert(thumbnails["small"], qt.Contains, "/small")
				c.Assert(thumbnails["medium"], qt.Contains, "/thumbnails/")
				c.Assert(thumbnails["medium"], qt.Contains, "/medium")
			} else {
				c.Assert(thumbnails, qt.HasLen, tt.expectedThumbnailCount)
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
				TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
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

			_, thumbnails, err := service.GenerateSignedURLsWithThumbnails(fileEntity, "user-id", "")
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

// TestExtractSessionBinding covers the helper that lifts the binding off
// the incoming request: nil request, no cookie, empty cookie, and a real
// cookie should all behave per spec.
func TestExtractSessionBinding(t *testing.T) {
	c := qt.New(t)

	c.Run("nil request returns empty", func(c *qt.C) {
		c.Assert(services.ExtractSessionBinding(nil), qt.Equals, services.SessionBinding(""))
	})

	c.Run("missing cookie returns empty", func(c *qt.C) {
		r := httptest.NewRequest(http.MethodGet, "/whatever", nil)
		c.Assert(services.ExtractSessionBinding(r), qt.Equals, services.SessionBinding(""))
	})

	c.Run("empty cookie value returns empty", func(c *qt.C) {
		r := httptest.NewRequest(http.MethodGet, "/whatever", nil)
		r.AddCookie(newTestRefreshCookie(""))
		c.Assert(services.ExtractSessionBinding(r), qt.Equals, services.SessionBinding(""))
	})

	c.Run("present cookie produces stable, non-empty binding", func(c *qt.C) {
		r1 := httptest.NewRequest(http.MethodGet, "/whatever", nil)
		r1.AddCookie(newTestRefreshCookie("the-cookie-value"))
		r2 := httptest.NewRequest(http.MethodGet, "/whatever", nil)
		r2.AddCookie(newTestRefreshCookie("the-cookie-value"))

		b1 := services.ExtractSessionBinding(r1)
		b2 := services.ExtractSessionBinding(r2)

		c.Assert(string(b1), qt.Not(qt.Equals), "")
		c.Assert(b1, qt.Equals, b2)
	})

	c.Run("different cookies produce different bindings", func(c *qt.C) {
		r1 := httptest.NewRequest(http.MethodGet, "/whatever", nil)
		r1.AddCookie(newTestRefreshCookie("cookie-A"))
		r2 := httptest.NewRequest(http.MethodGet, "/whatever", nil)
		r2.AddCookie(newTestRefreshCookie("cookie-B"))

		c.Assert(services.ExtractSessionBinding(r1), qt.Not(qt.Equals), services.ExtractSessionBinding(r2))
	})

	c.Run("other cookies do not contribute", func(c *qt.C) {
		r := httptest.NewRequest(http.MethodGet, "/whatever", nil)
		// #nosec G124 -- intentionally different cookie names; transport security irrelevant.
		r.AddCookie(&http.Cookie{Name: "session", Value: "looks-juicy", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
		// #nosec G124 -- intentionally different cookie names; transport security irrelevant.
		r.AddCookie(&http.Cookie{Name: "access_token", Value: "also-juicy", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
		c.Assert(services.ExtractSessionBinding(r), qt.Equals, services.SessionBinding(""))
	})
}

// TestFileSigningService_SessionBindingMatrix is the heart of #1781: the
// signature MUST only verify when the binding presented at validate time
// equals the binding the URL was minted with.
func TestFileSigningService_SessionBindingMatrix(t *testing.T) {
	signingKey := []byte("test-signing-key-32-bytes-long!!")
	expiration := 15 * time.Minute
	service := services.NewFileSigningService(signingKey, expiration)

	const fileID = "file-1"
	const fileExt = "pdf"
	const userID = "user-1"

	type row struct {
		name        string
		mintWith    services.SessionBinding
		validateAs  services.SessionBinding
		expectValid bool
	}

	rows := []row{
		{
			name:        "bound mint, same binding validates",
			mintWith:    "session-A",
			validateAs:  "session-A",
			expectValid: true,
		},
		{
			name:        "bound mint, different binding rejected",
			mintWith:    "session-A",
			validateAs:  "session-B",
			expectValid: false,
		},
		{
			name:        "bound mint, empty binding rejected",
			mintWith:    "session-A",
			validateAs:  "",
			expectValid: false,
		},
		{
			name:        "unbound mint, empty binding validates (back-compat)",
			mintWith:    "",
			validateAs:  "",
			expectValid: true,
		},
		{
			name:        "unbound mint, any binding rejected",
			mintWith:    "",
			validateAs:  "session-A",
			expectValid: false,
		},
	}

	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			c := qt.New(t)

			signed, err := service.GenerateSignedURL(fileID, fileExt, userID, r.mintWith)
			c.Assert(err, qt.IsNil)

			parsed, err := url.Parse(signed)
			c.Assert(err, qt.IsNil)

			// The binding must never appear in the URL — that is the
			// whole point of binding via cookie.
			c.Assert(parsed.Query().Get("binding"), qt.Equals, "")
			c.Assert(parsed.Query().Get("sb"), qt.Equals, "")

			claims, err := service.ValidateSignedURL(parsed.Path, parsed.Query(), r.validateAs)
			if r.expectValid {
				c.Assert(err, qt.IsNil)
				c.Assert(claims, qt.IsNotNil)
				c.Assert(claims.FileID, qt.Equals, fileID)
				c.Assert(claims.UserID, qt.Equals, userID)
			} else {
				c.Assert(err, qt.IsNotNil)
				c.Assert(err.Error(), qt.Equals, "invalid signature")
				c.Assert(claims, qt.IsNil)
			}
		})
	}

	// Extra row driven through the real ExtractSessionBinding helper, so a
	// future encoding-change regression in the helper (e.g. swapping
	// RawURLEncoding for URLEncoding, or shifting the truncation width)
	// cannot pass while the literal-string matrix above still does.
	t.Run("round-trip via ExtractSessionBinding helper", func(t *testing.T) {
		c := qt.New(t)

		mintReq := httptest.NewRequest(http.MethodGet, "/sign", nil)
		mintReq.AddCookie(newTestRefreshCookie("round-trip-cookie"))
		mintBinding := services.ExtractSessionBinding(mintReq)
		c.Assert(string(mintBinding), qt.Not(qt.Equals), "")

		signed, err := service.GenerateSignedURL(fileID, fileExt, userID, mintBinding)
		c.Assert(err, qt.IsNil)
		parsed, err := url.Parse(signed)
		c.Assert(err, qt.IsNil)

		// Same cookie at validate time → success.
		sameReq := httptest.NewRequest(http.MethodGet, parsed.Path+"?"+parsed.RawQuery, nil)
		sameReq.AddCookie(newTestRefreshCookie("round-trip-cookie"))
		_, err = service.ValidateSignedURL(parsed.Path, parsed.Query(), services.ExtractSessionBinding(sameReq))
		c.Assert(err, qt.IsNil)

		// No cookie at validate time → rejected.
		_, err = service.ValidateSignedURL(parsed.Path, parsed.Query(), services.ExtractSessionBinding(httptest.NewRequest(http.MethodGet, "/x", nil)))
		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Equals, "invalid signature")
	})
}

// TestFileSigningService_ThumbnailsCarryBinding ensures the thumbnail URL
// path also inherits the binding so a leaked thumbnail URL can't be used
// from a foreign session either.
func TestFileSigningService_ThumbnailsCarryBinding(t *testing.T) {
	c := qt.New(t)

	signingKey := []byte("test-signing-key-32-bytes-long!!")
	service := services.NewFileSigningService(signingKey, 15*time.Minute)

	fileEntity := &models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{ID: "img-1"},
		},
		Type: models.FileTypeImage,
		File: &models.File{
			Path:         "img-1",
			OriginalPath: "img-1.png",
			Ext:          ".png",
			MIMEType:     "image/png",
		},
	}

	originalURL, thumbnails, err := service.GenerateSignedURLsWithThumbnails(fileEntity, "user-1", "session-A")
	c.Assert(err, qt.IsNil)
	c.Assert(thumbnails, qt.HasLen, 2)

	// Original URL must accept the mint binding and reject foreign ones.
	parsed, err := url.Parse(originalURL)
	c.Assert(err, qt.IsNil)
	_, err = service.ValidateSignedURL(parsed.Path, parsed.Query(), "session-A")
	c.Assert(err, qt.IsNil)
	_, err = service.ValidateSignedURL(parsed.Path, parsed.Query(), "session-B")
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Equals, "invalid signature")

	// Each thumbnail URL must also reject foreign binding.
	for size, raw := range thumbnails {
		parsed, err := url.Parse(raw)
		c.Assert(err, qt.IsNil, qt.Commentf("thumbnail %q failed to parse", size))
		_, err = service.ValidateSignedURL(parsed.Path, parsed.Query(), "session-B")
		c.Assert(err, qt.IsNotNil, qt.Commentf("thumbnail %q should reject foreign binding", size))
		c.Assert(err.Error(), qt.Equals, "invalid signature")

		// And accept the original binding.
		_, err = service.ValidateSignedURL(parsed.Path, parsed.Query(), "session-A")
		c.Assert(err, qt.IsNil, qt.Commentf("thumbnail %q should accept original binding", size))
	}
}
