package oauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"
)

// DefaultStateTTL is the lifetime of a signed state token. Five minutes is
// generous enough to cover the slowest realistic provider round-trip (a
// user re-authenticating at Google with 2FA) and short enough that a
// stolen state value is useless before it can be replayed.
const DefaultStateTTL = 5 * time.Minute

// stateVersion is the version byte stamped into every state payload so a
// future incompatible change (e.g. swapping HMAC for AEAD) can be made
// without breaking in-flight callbacks at deploy time.
const stateVersion = 1

// State is the data signed into the OAuth `state` parameter for a single
// authorization-code roundtrip. The struct travels in two layers:
//
//  1. The PKCE Verifier rides inside the signed state rather than in a
//     separate cookie. This keeps the browser surface to a single signed
//     blob and means the callback can verify the entire round-trip from
//     one input.
//
//  2. The RedirectAfter field carries the FE's "where to send the user
//     after login" hint as a server-validated relative path. The handler
//     validates it against an allow-list of safe prefixes before issuing a
//     302 — never echoed back into the response without that check.
//
//  3. LinkUserID is non-empty only on the "link an additional provider to
//     this user" path (POST /auth/oauth/{provider}/link/start). The
//     callback uses its presence to skip the find-or-create branch and
//     attach the resulting identity to the existing user instead.
type State struct {
	Version  int    `json:"v"`
	Provider string `json:"p"`
	Nonce    string `json:"n"`
	// Verifier is the PKCE code_verifier (RFC 7636) for this round-trip.
	// PKCE §7.1 warns about leaving the verifier in URL-visible storage:
	// we accept that trade-off here in exchange for being stateless. The
	// state token is HMAC-signed (an attacker who intercepts it cannot
	// forge a fresh one) and SameSite=Lax + HttpOnly state cookie binding
	// pins the token to the browser that started the flow, so a leaked
	// verifier alone is not sufficient to complete an exchange. A future
	// move to server-side state storage (Redis or DB) would let us drop
	// the verifier from the URL; tracked under #1394 follow-ups.
	Verifier      string `json:"cv"`
	RedirectAfter string `json:"r,omitempty"`
	LinkUserID    string `json:"luid,omitempty"`
	IssuedAt      int64  `json:"iat"`
	ExpiresAt     int64  `json:"exp"`
}

// StateSigner produces and verifies HMAC-SHA256 signed state tokens for
// the OAuth authorize → callback roundtrip.
//
// Format: base64url(payload_json) + "." + base64url(hmac_sha256(payload_json)).
// Single-use replay protection is the caller's responsibility (compare the
// Nonce against a server-side store of consumed nonces or — what the
// handler does — bind the state cookie name to the nonce so a second
// callback for the same nonce arrives without a matching cookie). The
// signer itself only enforces signature integrity + expiry + version.
type StateSigner struct {
	key []byte
	ttl time.Duration
}

// NewStateSigner constructs a StateSigner keyed by key. The key must be at
// least 32 bytes; shorter keys are rejected so an operator misconfiguration
// can't downgrade to a low-entropy MAC.
func NewStateSigner(key []byte) (*StateSigner, error) {
	if len(key) < 32 {
		return nil, fmt.Errorf("oauth: state signing key must be at least 32 bytes (got %d)", len(key))
	}
	dup := make([]byte, len(key))
	copy(dup, key)
	return &StateSigner{key: dup, ttl: DefaultStateTTL}, nil
}

// WithTTL returns a copy of s with a custom token lifetime. Tests use this
// to mint an already-expired token.
func (s *StateSigner) WithTTL(ttl time.Duration) *StateSigner {
	if ttl <= 0 {
		ttl = DefaultStateTTL
	}
	return &StateSigner{key: s.key, ttl: ttl}
}

// NewNonce returns a fresh random nonce suitable for State.Nonce. Returned
// as a base64-url string so it's safe to embed in any HTTP context.
func NewNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("oauth: nonce rand.Read: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// Sign packs state into a signed, self-contained token. The token's
// IssuedAt and ExpiresAt are stamped from now() so the caller need not
// fill them; callers MUST populate Version=0 and let Sign promote it to
// the current stateVersion.
func (s *StateSigner) Sign(state State) (string, error) {
	now := time.Now()
	state.Version = stateVersion
	if state.IssuedAt == 0 {
		state.IssuedAt = now.Unix()
	}
	if state.ExpiresAt == 0 {
		state.ExpiresAt = now.Add(s.ttl).Unix()
	}
	if state.Nonce == "" {
		return "", fmt.Errorf("oauth: state.Nonce is required")
	}
	if state.Verifier == "" {
		return "", fmt.Errorf("oauth: state.Verifier is required")
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("oauth: marshal state: %w", err)
	}
	mac := hmac.New(sha256.New, s.key)
	mac.Write(payload)
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// ErrStateInvalid is returned whenever a token fails to verify for any
// reason (bad encoding, wrong signature, expired, version mismatch).
// Callers MUST NOT branch on the specific failure mode and MUST treat
// any error as "reject the callback" — leaking which check failed lets a
// caller probe the signing key one bit at a time.
var ErrStateInvalid = errors.New("oauth: invalid or expired state token")

// Verify parses and verifies token, returning the decoded State on
// success. Returns ErrStateInvalid on every failure mode.
func (s *StateSigner) Verify(token string) (State, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return State{}, ErrStateInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return State{}, ErrStateInvalid
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return State{}, ErrStateInvalid
	}
	mac := hmac.New(sha256.New, s.key)
	mac.Write(payload)
	expected := mac.Sum(nil)
	if !hmac.Equal(sig, expected) {
		return State{}, ErrStateInvalid
	}
	var st State
	if err := json.Unmarshal(payload, &st); err != nil {
		return State{}, ErrStateInvalid
	}
	if st.Version != stateVersion {
		return State{}, ErrStateInvalid
	}
	if st.ExpiresAt > 0 && time.Now().Unix() > st.ExpiresAt {
		return State{}, ErrStateInvalid
	}
	return st, nil
}

// SanitizeRedirect validates the FE-supplied `redirect=` query parameter
// against an open-redirect attack. Accepted: a relative path beginning
// with "/" but not "//" (the latter is a protocol-relative URL that the
// browser would resolve against the attacker's origin). Anything else
// returns the empty string, which the caller maps onto the safe default
// ("/").
//
// We deliberately resolve and clean the path so a payload like
// `/foo/../../evil` collapses to `/evil` (still safe) rather than being
// echoed verbatim.
func SanitizeRedirect(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// Reject protocol-relative ("//evil.com/...") and absolute URLs.
	if strings.HasPrefix(raw, "//") {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.Scheme != "" || u.Host != "" {
		return ""
	}
	if !strings.HasPrefix(u.Path, "/") {
		return ""
	}
	// Collapse `.` and `..` so a payload like `/foo/../../evil` cannot
	// climb above the FE app root. path.Clean preserves a leading slash.
	cleaned := path.Clean(u.Path)
	if !strings.HasPrefix(cleaned, "/") {
		return ""
	}
	// Preserve any query / fragment the caller wanted to land on.
	out := cleaned
	if u.RawQuery != "" {
		out += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		out += "#" + u.Fragment
	}
	return out
}
