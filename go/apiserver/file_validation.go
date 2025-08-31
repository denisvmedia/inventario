package apiserver

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
)

// FileValidationError represents file validation errors
type FileValidationError struct {
	Type    string
	Message string
	Field   string
}

func (e FileValidationError) Error() string {
	return fmt.Sprintf("file validation error [%s]: %s", e.Type, e.Message)
}

// File size limits (in bytes)
const (
	MaxImageFileSize   = 10 * 1024 * 1024  // 10MB for images
	MaxDocumentSize    = 50 * 1024 * 1024  // 50MB for documents
	MaxGeneralFileSize = 100 * 1024 * 1024 // 100MB for general files
)

// Allowed MIME types for different file categories
var (
	AllowedImageMimeTypes = map[string]bool{
		"image/jpeg":    true,
		"image/jpg":     true,
		"image/png":     true,
		"image/gif":     true,
		"image/webp":    true,
		"image/svg+xml": true,
	}

	AllowedDocumentMimeTypes = map[string]bool{
		"application/pdf":    true,
		"application/msword": true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
		"application/vnd.ms-excel": true,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
		"application/vnd.ms-powerpoint":                                             true,
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
		"text/plain":      true,
		"text/csv":        true,
		"application/rtf": true,
	}

	// Dangerous MIME types that should never be allowed
	DangerousMimeTypes = map[string]bool{
		"application/x-executable":    true,
		"application/x-msdownload":    true,
		"application/x-msdos-program": true,
		"application/x-msi":           true,
		"application/x-bat":           true,
		"application/x-sh":            true,
		"application/x-shellscript":   true,
		"application/javascript":      true,
		"text/javascript":             true,
		"application/x-php":           true,
		"text/x-php":                  true,
		"application/x-python-code":   true,
		"text/x-python":               true,
		"application/x-perl":          true,
		"text/x-perl":                 true,
		"application/x-ruby":          true,
		"text/x-ruby":                 true,
	}

	// Dangerous file extensions
	DangerousExtensions = map[string]bool{
		".exe": true, ".bat": true, ".cmd": true, ".com": true, ".pif": true,
		".scr": true, ".vbs": true, ".vbe": true, ".js": true, ".jse": true,
		".wsf": true, ".wsh": true, ".msi": true, ".msp": true, ".hta": true,
		".cpl": true, ".jar": true, ".app": true, ".deb": true, ".dmg": true,
		".pkg": true, ".rpm": true, ".sh": true, ".php": true, ".py": true,
		".pl": true, ".rb": true, ".asp": true, ".aspx": true, ".jsp": true,
	}
)

// ValidateFileUpload performs comprehensive file validation
func ValidateFileUpload(r *http.Request, filename string, fileSize int64, mimeType string, fileCategory string) *FileValidationError {
	// Log file upload attempt for security monitoring
	user := GetUserFromRequest(r)
	userID := ""
	if user != nil {
		userID = user.ID
	}

	slog.Info("File upload validation",
		"user_id", userID,
		"filename", filename,
		"file_size", fileSize,
		"mime_type", mimeType,
		"category", fileCategory,
		"remote_addr", r.RemoteAddr,
		"user_agent", r.UserAgent(),
	)

	// 1. Validate filename
	if err := validateFilename(filename); err != nil {
		logFileValidationViolation(r, "invalid_filename", err.Message, filename, mimeType, fileSize)
		return err
	}

	// 2. Validate file extension
	if err := validateFileExtension(filename); err != nil {
		logFileValidationViolation(r, "dangerous_extension", err.Message, filename, mimeType, fileSize)
		return err
	}

	// 3. Validate MIME type
	if err := validateMimeType(mimeType, filename); err != nil {
		logFileValidationViolation(r, "invalid_mime_type", err.Message, filename, mimeType, fileSize)
		return err
	}

	// 4. Validate file size based on category
	if err := validateFileSize(fileSize, fileCategory, mimeType); err != nil {
		logFileValidationViolation(r, "file_too_large", err.Message, filename, mimeType, fileSize)
		return err
	}

	// 5. Cross-validate MIME type and extension
	if err := validateMimeExtensionConsistency(mimeType, filename); err != nil {
		logFileValidationViolation(r, "mime_extension_mismatch", err.Message, filename, mimeType, fileSize)
		return err
	}

	slog.Debug("File upload validation passed",
		"user_id", userID,
		"filename", filename,
		"file_size", fileSize,
		"mime_type", mimeType,
	)

	return nil
}

// validateFilename checks for path traversal and invalid characters
func validateFilename(filename string) *FileValidationError {
	if filename == "" {
		return &FileValidationError{
			Type:    "empty_filename",
			Message: "Filename cannot be empty",
			Field:   "filename",
		}
	}

	// Check for path traversal attempts
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return &FileValidationError{
			Type:    "path_traversal",
			Message: "Filename contains invalid path characters",
			Field:   "filename",
		}
	}

	// Check for null bytes and control characters
	for _, char := range filename {
		if char < 32 && char != 9 { // Allow tab but not other control chars
			return &FileValidationError{
				Type:    "invalid_characters",
				Message: "Filename contains invalid control characters",
				Field:   "filename",
			}
		}
	}

	// Check filename length
	if len(filename) > 255 {
		return &FileValidationError{
			Type:    "filename_too_long",
			Message: "Filename exceeds maximum length of 255 characters",
			Field:   "filename",
		}
	}

	return nil
}

// validateFileExtension checks for dangerous file extensions
func validateFileExtension(filename string) *FileValidationError {
	ext := strings.ToLower(filepath.Ext(filename))

	if DangerousExtensions[ext] {
		return &FileValidationError{
			Type:    "dangerous_extension",
			Message: fmt.Sprintf("File extension '%s' is not allowed for security reasons", ext),
			Field:   "filename",
		}
	}

	return nil
}

// validateMimeType checks if the MIME type is allowed
func validateMimeType(mimeType, filename string) *FileValidationError {
	if mimeType == "" {
		return &FileValidationError{
			Type:    "missing_mime_type",
			Message: "MIME type is required",
			Field:   "mime_type",
		}
	}

	// Check for dangerous MIME types
	if DangerousMimeTypes[strings.ToLower(mimeType)] {
		return &FileValidationError{
			Type:    "dangerous_mime_type",
			Message: fmt.Sprintf("MIME type '%s' is not allowed for security reasons", mimeType),
			Field:   "mime_type",
		}
	}

	// Normalize MIME type
	normalizedMime := strings.ToLower(strings.TrimSpace(mimeType))

	// Check if it's a known safe MIME type
	if AllowedImageMimeTypes[normalizedMime] || AllowedDocumentMimeTypes[normalizedMime] {
		return nil
	}

	// For other MIME types, be more restrictive
	if strings.HasPrefix(normalizedMime, "text/") &&
		(normalizedMime == "text/plain" || normalizedMime == "text/csv") {
		return nil
	}

	return &FileValidationError{
		Type:    "unsupported_mime_type",
		Message: fmt.Sprintf("MIME type '%s' is not supported", mimeType),
		Field:   "mime_type",
	}
}

// validateFileSize checks if file size is within limits
func validateFileSize(fileSize int64, category, mimeType string) *FileValidationError {
	if fileSize <= 0 {
		return &FileValidationError{
			Type:    "invalid_file_size",
			Message: "File size must be greater than 0",
			Field:   "file_size",
		}
	}

	var maxSize int64
	switch category {
	case "image":
		maxSize = MaxImageFileSize
	case "document", "manual", "invoice":
		maxSize = MaxDocumentSize
	default:
		maxSize = MaxGeneralFileSize
	}

	// Additional size limits based on MIME type
	if AllowedImageMimeTypes[strings.ToLower(mimeType)] && fileSize > MaxImageFileSize {
		return &FileValidationError{
			Type:    "file_too_large",
			Message: fmt.Sprintf("Image file size (%d bytes) exceeds maximum allowed size (%d bytes)", fileSize, MaxImageFileSize),
			Field:   "file_size",
		}
	}

	if fileSize > maxSize {
		return &FileValidationError{
			Type:    "file_too_large",
			Message: fmt.Sprintf("File size (%d bytes) exceeds maximum allowed size (%d bytes)", fileSize, maxSize),
			Field:   "file_size",
		}
	}

	return nil
}

// validateMimeExtensionConsistency checks if MIME type matches file extension
func validateMimeExtensionConsistency(mimeType, filename string) *FileValidationError {
	ext := strings.ToLower(filepath.Ext(filename))
	normalizedMime := strings.ToLower(strings.TrimSpace(mimeType))

	// Check for obvious mismatches between MIME type and extension
	mismatchCases := map[string][]string{
		"image/":          {".exe", ".bat", ".cmd", ".php", ".js", ".html"},
		"text/":           {".exe", ".bat", ".cmd", ".jpg", ".png", ".gif"},
		"application/pdf": {".exe", ".bat", ".cmd", ".jpg", ".png", ".gif", ".js", ".php"},
	}

	for mimePrefix, dangerousExts := range mismatchCases {
		if strings.HasPrefix(normalizedMime, mimePrefix) {
			for _, dangerousExt := range dangerousExts {
				if ext == dangerousExt {
					return &FileValidationError{
						Type:    "mime_extension_mismatch",
						Message: fmt.Sprintf("MIME type '%s' does not match file extension '%s'", mimeType, ext),
						Field:   "mime_type",
					}
				}
			}
		}
	}

	return nil
}

// logFileValidationViolation logs file validation violations for security monitoring
func logFileValidationViolation(r *http.Request, violationType, message, filename, mimeType string, fileSize int64) {
	user := GetUserFromRequest(r)
	userID := ""
	tenantID := ""
	if user != nil {
		userID = user.ID
		tenantID = user.TenantID
	}

	slog.Warn("File validation violation",
		"violation_type", violationType,
		"message", message,
		"filename", filename,
		"mime_type", mimeType,
		"file_size", fileSize,
		"user_id", userID,
		"tenant_id", tenantID,
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
		"user_agent", r.UserAgent(),
	)
}
