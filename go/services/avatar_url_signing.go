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

// AvatarURLPrefix is the route signed avatar URLs are served from. The subject
// user's ID follows it.
const AvatarURLPrefix = "/api/v1/avatars/"

// AvatarURLClaims is what a valid signed avatar URL grants.
type AvatarURLClaims struct {
	// SubjectID is the user whose photo the URL shows.
	SubjectID string
	// ViewerID is the user the URL was minted for. Empty on a URL minted for
	// a back-office operator.
	ViewerID string
	// OperatorID is the back-office operator the URL was minted for, see
	// GenerateOperatorAvatarURL. Empty on a URL minted for a user.
	OperatorID string
	// GroupID is the group the viewer and the subject shared when the URL was
	// minted. Empty when the viewer is the subject.
	GroupID string
	// Version is the stored key's version the URL was minted for, see
	// AvatarVersion.
	Version   string
	ExpiresAt time.Time
}

// GenerateAvatarURL mints a signed URL for a user's profile photo, so an <img>
// can load it without the bearer token, which a browser does not send for one.
//
// The caller decides that the viewer may see the subject's photo before it
// mints the URL: either they are the same user, or both are members of
// groupID. The group is signed into the URL so the serving side can check the
// same thing again, because a membership can end before the URL expires.
//
// Unlike a file URL, the expiry is rounded up to a whole multiple of the
// configured expiration, and the URL stays valid for between one and two of
// those periods. The point is a stable URL: the same photo requested twice in
// one period gets byte-identical URLs, so the browser cache serves the second
// request and an avatar does not flicker each time a list refetches.
func (s *FileSigningService) GenerateAvatarURL(subjectID, avatarKey, viewerID, groupID string, binding SessionBinding) (string, error) {
	if subjectID == "" || viewerID == "" {
		return "", errors.New("subject and viewer IDs are required")
	}
	if avatarKey == "" {
		return "", errors.New("avatar key is required")
	}
	if groupID == "" && viewerID != subjectID {
		return "", errors.New("a group is required to sign another user's avatar")
	}

	version := AvatarVersion(avatarKey)
	exp := s.avatarURLExpiry(time.Now())
	// The message covers the decoded path, which is what the serving side reads
	// from the request; the URL carries the escaped one.
	signature, err := s.generateSignature(avatarURLMessage(AvatarURLPrefix+subjectID, viewerID, groupID, version, exp, binding))
	if err != nil {
		return "", errxtrace.Wrap("failed to sign the avatar URL", err)
	}

	query := url.Values{}
	query.Set("v", version)
	query.Set("uid", viewerID)
	if groupID != "" {
		query.Set("gid", groupID)
	}
	query.Set("exp", strconv.FormatInt(exp, 10))
	query.Set("sig", signature)
	return AvatarURLPrefix + url.PathEscape(subjectID) + "?" + query.Encode(), nil
}

// GenerateOperatorAvatarURL mints a signed URL for a user's profile photo
// that a back-office operator may load: the admin pages show the users and
// group members of every tenant. The caller has checked that operatorID is
// the authenticated operator. The URL names the operator in `op` instead of
// a viewer in `uid`, and its HMAC covers a message of its own, so neither
// kind of URL can be turned into the other. Expiry and caching work as for
// GenerateAvatarURL.
func (s *FileSigningService) GenerateOperatorAvatarURL(subjectID, avatarKey, operatorID string, binding SessionBinding) (string, error) {
	if subjectID == "" || operatorID == "" {
		return "", errors.New("subject and operator IDs are required")
	}
	if avatarKey == "" {
		return "", errors.New("avatar key is required")
	}

	version := AvatarVersion(avatarKey)
	exp := s.avatarURLExpiry(time.Now())
	signature, err := s.generateSignature(operatorAvatarURLMessage(AvatarURLPrefix+subjectID, operatorID, version, exp, binding))
	if err != nil {
		return "", errxtrace.Wrap("failed to sign the avatar URL", err)
	}

	query := url.Values{}
	query.Set("v", version)
	query.Set("op", operatorID)
	query.Set("exp", strconv.FormatInt(exp, 10))
	query.Set("sig", signature)
	return AvatarURLPrefix + url.PathEscape(subjectID) + "?" + query.Encode(), nil
}

// ValidateAvatarURL checks a signed avatar URL and returns what it grants.
// urlPath is the decoded request path (r.URL.Path); binding comes from the
// validating request, exactly as for file URLs (see ExtractSessionBinding).
// A URL minted by GenerateOperatorAvatarURL returns claims with OperatorID
// set and ViewerID empty.
func (s *FileSigningService) ValidateAvatarURL(urlPath string, query url.Values, binding SessionBinding) (*AvatarURLClaims, error) {
	subjectID, ok := strings.CutPrefix(urlPath, AvatarURLPrefix)
	if !ok || subjectID == "" || strings.Contains(subjectID, "/") {
		return nil, errors.New("not an avatar URL")
	}

	signature := query.Get("sig")
	version := query.Get("v")
	viewerID := query.Get("uid")
	operatorID := query.Get("op")
	groupID := query.Get("gid")
	if signature == "" || version == "" || (viewerID == "") == (operatorID == "") {
		return nil, errors.New("missing avatar URL parameter")
	}
	if operatorID != "" && groupID != "" {
		return nil, errors.New("unexpected avatar URL parameter")
	}
	exp, err := strconv.ParseInt(query.Get("exp"), 10, 64)
	if err != nil {
		return nil, errors.New("invalid expiration timestamp")
	}
	expiresAt := time.Unix(exp, 0)
	if time.Now().After(expiresAt) {
		return nil, errors.New("signed URL has expired")
	}

	message := avatarURLMessage(urlPath, viewerID, groupID, version, exp, binding)
	if operatorID != "" {
		message = operatorAvatarURLMessage(urlPath, operatorID, version, exp, binding)
	}
	if !s.validateSignature(message, signature) {
		return nil, errors.New("invalid signature")
	}
	return &AvatarURLClaims{
		SubjectID:  subjectID,
		ViewerID:   viewerID,
		OperatorID: operatorID,
		GroupID:    groupID,
		Version:    version,
		ExpiresAt:  expiresAt,
	}, nil
}

// avatarURLMessage is the string the avatar URL's HMAC covers. The leading
// AVATAR keeps it from ever equaling a file URL's message, which starts with
// the method.
func avatarURLMessage(urlPath, viewerID, groupID, version string, exp int64, binding SessionBinding) string {
	return fmt.Sprintf("AVATAR|GET|%s|%s|%s|%s|%d|%s", urlPath, viewerID, groupID, version, exp, binding)
}

// operatorAvatarURLMessage is the string an operator avatar URL's HMAC
// covers. The OPERATOR field keeps it from ever equaling a user URL's
// message, so an operator ID cannot be passed off as a user ID or back.
func operatorAvatarURLMessage(urlPath, operatorID, version string, exp int64, binding SessionBinding) string {
	return fmt.Sprintf("AVATAR|OPERATOR|GET|%s|%s|%s|%d|%s", urlPath, operatorID, version, exp, binding)
}

// avatarURLExpiry returns the second period boundary after now: between one
// and two periods away, and the same for every call within a period.
func (s *FileSigningService) avatarURLExpiry(now time.Time) int64 {
	period := max(int64(s.expiration/time.Second), 1)
	return (now.Unix()/period + 2) * period
}
