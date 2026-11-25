package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// APITokenPrefix starts every personal API token. A JWT access token always
// starts with "eyJ" (a base64url-encoded JSON header) and carries two dots, so
// the two credential kinds cannot be confused by a parser, and a secret scanner
// can match this prefix alone.
const APITokenPrefix = "inv_pat_"

// apiTokenSecretLen is the number of base62 characters after the prefix.
// 43 characters at log2(62) ≈ 5.95 bits each carry 256 bits of entropy.
const apiTokenSecretLen = 43

// APITokenLength is the full length of a well-formed token, prefix included.
const APITokenLength = len(APITokenPrefix) + apiTokenSecretLen

// apiTokenDisplayLen is how much of the token is kept in clear for the token
// list: the prefix plus the first four secret characters. Four base62
// characters reveal about 24 of the 256 bits.
const apiTokenDisplayLen = len(APITokenPrefix) + 4

const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// APITokenScope is what a personal API token may do. The scope is enforced by
// the server on every request, by HTTP method.
type APITokenScope string

const (
	// APITokenScopeReadOnly allows GET, HEAD and OPTIONS only.
	APITokenScopeReadOnly APITokenScope = "read_only"
	// APITokenScopeReadWrite allows every method the owner's group role allows.
	APITokenScopeReadWrite APITokenScope = "read_write"
)

// IsValid reports whether the scope is one of the known values.
func (s APITokenScope) IsValid() bool {
	return s == APITokenScopeReadOnly || s == APITokenScopeReadWrite
}

// Row-level security mirrors refresh_tokens: the application role sees only the
// caller's own rows. Token lookup by hash happens before any user context
// exists, so the registry runs under the background-worker role, which the
// second policy admits.
//
//ptah:schema:rls:enable table="api_tokens" comment="Enable RLS for multi-tenant API token isolation"
//ptah:schema:rls:policy name="api_token_isolation" table="api_tokens" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" comment="Ensures API tokens can only be accessed and modified by the owning user within their tenant"
//ptah:schema:rls:policy name="api_token_background_worker_access" table="api_tokens" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows token authentication to look tokens up by hash before a user context exists"

// APIToken is a personal API token: a long-lived bearer credential a user
// creates for a script or another application. Only the SHA-256 digest of the
// token is stored; the token itself is shown once, in the response that
// creates it.
//
//ptah:schema:table name="api_tokens"
type APIToken struct {
	//ptah:embedded mode="inline"
	TenantUserAwareEntityID
	// Name is the label the user gave the token, e.g. "Home Assistant".
	//ptah:schema:field name="name" type="VARCHAR(100)" not_null="true"
	Name string `json:"name" db:"name"`
	// TokenHash is the hex SHA-256 digest of the whole token. The token has
	// 256 bits of entropy, so a fast digest is enough: there is nothing to
	// brute-force, and a slow hash would only add latency to every request.
	//ptah:schema:field name="token_hash" type="VARCHAR(64)" not_null="true"
	TokenHash string `json:"-" db:"token_hash"`
	// TokenPrefix is the start of the token kept in clear so a user can tell
	// their tokens apart, e.g. "inv_pat_AbCd".
	//ptah:schema:field name="token_prefix" type="VARCHAR(16)" not_null="true"
	TokenPrefix string `json:"token_prefix" db:"token_prefix"`
	//ptah:schema:field name="scope" type="VARCHAR(16)" not_null="true"
	Scope APITokenScope `json:"scope" db:"scope"`
	// GroupID restricts the token to one group when set. ON DELETE CASCADE:
	// removing the group must remove the token, never widen it into an
	// unrestricted one, which SET NULL would do.
	//ptah:schema:field name="group_id" type="TEXT" foreign="location_groups(id)" foreign_key_name="fk_api_token_group" on_delete="CASCADE"
	GroupID *string `json:"group_id,omitempty" db:"group_id"`
	// ExpiresAt is nil for a token that does not expire.
	//ptah:schema:field name="expires_at" type="TIMESTAMP"
	ExpiresAt *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	//ptah:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	// LastUsedAt is updated at most once per APITokenLastUsedInterval, so it
	// is accurate to that interval and not to the request.
	//ptah:schema:field name="last_used_at" type="TIMESTAMP"
	LastUsedAt *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
	//ptah:schema:field name="revoked_at" type="TIMESTAMP"
	RevokedAt *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
}

// APITokenIndexes defines PostgreSQL indexes for the api_tokens table.
type APITokenIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore)
	//ptah:schema:index name="idx_api_tokens_uuid" fields="uuid" unique="true" table="api_tokens"
	_ int

	// Every authenticated token request looks the row up by digest.
	//ptah:schema:index name="idx_api_tokens_token_hash" fields="token_hash" unique="true" table="api_tokens"
	_ int

	// The token list and the revoke-all paths select by owner.
	//ptah:schema:index name="idx_api_tokens_user_id" fields="user_id" table="api_tokens"
	_ int
}

// APITokenLastUsedInterval is how stale last_used_at may get before a request
// writes it again. Writing it on every request would turn every read into a
// database write.
const APITokenLastUsedInterval = 5 * time.Minute

// IsExpired reports whether the token has an expiry and it has passed.
func (t *APIToken) IsExpired(now time.Time) bool {
	return t.ExpiresAt != nil && !now.Before(*t.ExpiresAt)
}

// IsRevoked reports whether the token was revoked.
func (t *APIToken) IsRevoked() bool {
	return t.RevokedAt != nil
}

// IsUsable reports whether the token may authenticate a request at now.
func (t *APIToken) IsUsable(now time.Time) bool {
	return !t.IsRevoked() && !t.IsExpired(now)
}

// AllowsMethod reports whether the token's scope permits the HTTP method.
// read_only permits the safe methods, read_write any method, and a scope this
// server does not know permits nothing.
func (t *APIToken) AllowsMethod(method string) bool {
	switch t.Scope {
	case APITokenScopeReadWrite:
		return true
	case APITokenScopeReadOnly:
		switch method {
		case "GET", "HEAD", "OPTIONS":
			return true
		}
	}
	return false
}

// GeneratedAPIToken is a freshly minted token: the value handed to the user
// once, the digest that is stored, and the prefix kept in clear for display.
type GeneratedAPIToken struct {
	Token         string
	Hash          string
	DisplayPrefix string
}

// GenerateAPIToken mints a new token.
func GenerateAPIToken() (GeneratedAPIToken, error) {
	secret, err := randomBase62(apiTokenSecretLen)
	if err != nil {
		return GeneratedAPIToken{}, err
	}
	token := APITokenPrefix + secret
	return GeneratedAPIToken{
		Token:         token,
		Hash:          HashAPIToken(token),
		DisplayPrefix: token[:apiTokenDisplayLen],
	}, nil
}

// HashAPIToken returns the hex SHA-256 digest of a token.
func HashAPIToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// LooksLikeAPIToken reports whether a bearer credential is meant to be a
// personal API token. It checks the prefix only, so a mistyped token is still
// routed to the token path and rejected there, rather than being parsed as a
// JWT.
func LooksLikeAPIToken(credential string) bool {
	return strings.HasPrefix(credential, APITokenPrefix)
}

// IsWellFormedAPIToken reports whether a credential has the exact length and
// alphabet of a token this server mints. A malformed token is rejected without
// a database lookup.
func IsWellFormedAPIToken(credential string) bool {
	if len(credential) != APITokenLength || !LooksLikeAPIToken(credential) {
		return false
	}
	for i := len(APITokenPrefix); i < len(credential); i++ {
		if strings.IndexByte(base62Alphabet, credential[i]) < 0 {
			return false
		}
	}
	return true
}

// randomBase62 draws n uniformly distributed base62 characters from
// crypto/rand. Bytes at or above 248 (the largest multiple of 62 that fits in
// a byte) are rejected so the modulo does not favor the first characters.
func randomBase62(n int) (string, error) {
	const limit = 256 - 256%len(base62Alphabet)
	out := make([]byte, 0, n)
	buf := make([]byte, n+8)
	for len(out) < n {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, b := range buf {
			if int(b) >= limit {
				continue
			}
			out = append(out, base62Alphabet[int(b)%len(base62Alphabet)])
			if len(out) == n {
				break
			}
		}
	}
	return string(out), nil
}
