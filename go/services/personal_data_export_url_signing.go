package services

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
)

// PersonalDataExportURLPrefix is the route signed personal data export
// download URLs are served from (#2177). The export's ID follows it.
const PersonalDataExportURLPrefix = "/api/v1/data-exports/"

// PersonalDataExportURLClaims is what a valid signed download URL grants.
type PersonalDataExportURLClaims struct {
	ExportID  string
	UserID    string
	ExpiresAt time.Time
}

// GeneratePersonalDataExportURL mints a signed URL to download a personal
// data export, so the browser can save the archive with a plain link instead
// of reading it into memory through an authenticated request. The caller
// checks that userID owns the export and that it is ready. The URL is bound
// to the session that asked for it, like a file URL, and expires after the
// configured file URL lifetime.
func (s *FileSigningService) GeneratePersonalDataExportURL(exportID, userID string, binding SessionBinding) (string, time.Time, error) {
	if exportID == "" || userID == "" {
		return "", time.Time{}, errors.New("export and user IDs are required")
	}
	expiresAt := time.Now().Add(s.expiration).Truncate(time.Second)
	exp := expiresAt.Unix()
	signature, err := s.generateSignature(personalDataExportURLMessage(PersonalDataExportURLPrefix+exportID, userID, exp, binding))
	if err != nil {
		return "", time.Time{}, errxtrace.Wrap("failed to sign the download URL", err)
	}
	query := url.Values{}
	query.Set("uid", userID)
	query.Set("exp", strconv.FormatInt(exp, 10))
	query.Set("sig", signature)
	return PersonalDataExportURLPrefix + url.PathEscape(exportID) + "?" + query.Encode(), expiresAt, nil
}

// ValidatePersonalDataExportURL checks a signed download URL and returns what
// it grants. urlPath is the decoded request path; binding comes from the
// validating request (see ExtractSessionBinding).
func (s *FileSigningService) ValidatePersonalDataExportURL(urlPath string, query url.Values, binding SessionBinding) (*PersonalDataExportURLClaims, error) {
	exportID, ok := strings.CutPrefix(urlPath, PersonalDataExportURLPrefix)
	if !ok || exportID == "" || strings.Contains(exportID, "/") {
		return nil, errors.New("not a personal data export URL")
	}
	signature := query.Get("sig")
	userID := query.Get("uid")
	if signature == "" || userID == "" {
		return nil, errors.New("missing download URL parameter")
	}
	exp, err := strconv.ParseInt(query.Get("exp"), 10, 64)
	if err != nil {
		return nil, errors.New("invalid expiration timestamp")
	}
	expiresAt := time.Unix(exp, 0)
	if time.Now().After(expiresAt) {
		return nil, errors.New("signed URL has expired")
	}
	if !s.validateSignature(personalDataExportURLMessage(urlPath, userID, exp, binding), signature) {
		return nil, errors.New("invalid signature")
	}
	return &PersonalDataExportURLClaims{ExportID: exportID, UserID: userID, ExpiresAt: expiresAt}, nil
}

// personalDataExportURLMessage is the string the download URL's HMAC covers.
// The leading tag keeps it from ever equaling a file or avatar URL's message.
func personalDataExportURLMessage(urlPath, userID string, exp int64, binding SessionBinding) string {
	return fmt.Sprintf("PERSONAL_DATA_EXPORT|GET|%s|%s|%d|%s", urlPath, userID, exp, binding)
}
