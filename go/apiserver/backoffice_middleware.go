package apiserver

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// RequireBackofficeAuth validates a back-office (aud="backoffice") JWT
// and attaches the corresponding BackofficeUser to the request context.
// It MUST be the only authentication middleware mounted on back-office
// routes — using JWTMiddleware would happily accept a tenant token,
// defeating the cross-plane boundary.
//
// The middleware enforces three independent guards:
//
//  1. `aud == "backoffice"` — anything missing or set to anything else
//     is rejected, including the historical no-aud tenant tokens minted
//     before tenant-side `aud` stamping landed.
//  2. The `admin_id` claim must be present and non-empty. Pure tenant
//     tokens carry `user_id` instead; rejecting requests whose admin_id
//     is empty hardens the plane against a misconfigured mint that ever
//     forgot to stamp the claim.
//  3. `user_id` claim must NOT be present. A token carrying both
//     claims is treated as forged/misminted — back-office tokens never
//     stamp user_id, so its presence is proof of misuse.
//
// In addition to the JWT checks the middleware loads the back-office
// user from the registry on every request — same as JWTMiddleware does
// for tenant users — and rejects inactive accounts.
//
// blacklist may be nil; pass non-nil to enable jti/user blacklist
// checks. The user-level blacklist key is prefixed with
// "backoffice:" so the blacklist namespaces of the two planes can
// never accidentally collide.
func RequireBackofficeAuth(jwtSecret []byte, backofficeUserRegistry registry.BackofficeUserRegistry, blacklist services.TokenBlacklister) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString, err := extractTokenFromRequest(r)
			if err != nil {
				slog.Warn("Backoffice auth: missing/invalid Authorization header", "path", r.URL.Path)
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			claims, err := validateBackofficeJWT(r.Context(), tokenString, jwtSecret, blacklist)
			if err != nil {
				slog.Warn("Backoffice auth: token validation failed", "path", r.URL.Path, "error", err)
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			adminID, _ := claims["admin_id"].(string)
			// adminID emptiness already rejected by validateBackofficeJWT,
			// but re-check defensively before the lookup to keep the
			// failure mode explicit if validation ever loosens.
			if adminID == "" {
				slog.Warn("Backoffice auth: token missing admin_id", "path", r.URL.Path)
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			user, err := backofficeUserRegistry.Get(r.Context(), adminID)
			if err != nil {
				slog.Warn("Backoffice auth: user not found", "admin_id", adminID, "error", err)
				http.Error(w, "user not found", http.StatusUnauthorized)
				return
			}
			if !user.IsActive {
				slog.Warn("Backoffice auth: account disabled", "admin_id", adminID)
				http.Error(w, "account disabled", http.StatusForbidden)
				return
			}

			ctx := appctx.WithBackofficeUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// validateBackofficeJWT parses and validates a JWT minted by the
// back-office auth plane. Distinct from validateJWTToken on three axes:
//
//   - Enforces `aud == "backoffice"`.
//   - Requires `admin_id`, rejects `user_id`.
//   - User-blacklist lookups are namespaced by `backoffice:` so the
//     blacklist universe never aliases the tenant universe.
//
// Returns the claims on success.
func validateBackofficeJWT(ctx context.Context, tokenString string, jwtSecret []byte, blacklist services.TokenBlacklister) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	exp, hasExp := claims["exp"].(float64)
	if !hasExp {
		return nil, fmt.Errorf("token missing expiration claim")
	}
	if int64(exp) <= time.Now().Unix() {
		return nil, fmt.Errorf("token expired")
	}

	if aud, _ := claims["aud"].(string); aud != backofficeTokenAudience {
		// A tenant token (no aud, or aud="tenant") MUST never resolve to
		// a back-office identity. This is the primary cross-plane guard.
		return nil, fmt.Errorf("invalid token audience")
	}

	// admin_id presence is mandatory.
	adminID, _ := claims["admin_id"].(string)
	if adminID == "" {
		return nil, fmt.Errorf("token missing admin_id claim")
	}

	// user_id MUST NOT be present on a back-office token. If it ever is,
	// the token was either forged or misminted — reject either way.
	if userID, ok := claims["user_id"].(string); ok && userID != "" {
		return nil, fmt.Errorf("token carries tenant user_id claim")
	}

	// token_type guard mirrors the tenant validateJWTToken — only the
	// access type is accepted; the future MFA challenge token (Phase 4)
	// has a separate type and a separate endpoint.
	tokenType, _ := claims["token_type"].(string)
	if tokenType != accessTokenType {
		return nil, fmt.Errorf("invalid token type")
	}

	if blacklist != nil {
		if err := checkBackofficeTokenBlacklist(ctx, claims, blacklist); err != nil {
			return nil, err
		}
	}

	return claims, nil
}

// checkBackofficeTokenBlacklist runs both the per-token (jti) and
// per-user blacklist checks. The user-key namespace is `backoffice:` so
// it cannot collide with tenant keys.
//
// Fail-open on backend errors (mirroring checkTokenBlacklist): a Redis
// outage must not lock back-office operators out either.
func checkBackofficeTokenBlacklist(ctx context.Context, claims jwt.MapClaims, blacklist services.TokenBlacklister) error {
	if jti, ok := claims["jti"].(string); ok && jti != "" {
		blacklisted, err := blacklist.IsBlacklisted(ctx, jti)
		if err != nil {
			slog.Error("Failed to check backoffice token blacklist", "error", err)
		} else if blacklisted {
			return fmt.Errorf("token has been revoked")
		}
	}
	if adminID, ok := claims["admin_id"].(string); ok && adminID != "" {
		key := backofficeBlacklistUserKey(adminID)
		since, blacklisted, err := blacklist.UserBlacklistedSince(ctx, key)
		if err != nil {
			slog.Error("Failed to check backoffice user blacklist", "error", err)
		} else if blacklisted {
			return checkBackofficeUserBlacklistIat(claims, since)
		}
	}
	return nil
}

// AdminRoleRequiredCode is the JSON:API error code returned when a
// back-office route gated on RequirePlatformAdmin is hit by a
// back-office user whose role is not platform_admin (e.g. a
// support_agent attempting to start an impersonation session). Kept as
// a constant so the FE branch table and the tests reference the same
// literal; lives in the `admin.*` namespace alongside
// `admin.forbidden` for consistency.
const AdminRoleRequiredCode = "admin.role_required"

// RequirePlatformAdmin gates a back-office route subtree on a
// platform_admin role. MUST run AFTER RequireBackofficeAuth so the
// back-office identity is already in the context. A support_agent (the
// read-mostly persona) reaching this point is the most common rejection
// case — turning the start-impersonation surface into platform_admin
// only is the primary use of this middleware in Phase 5 of issue #1785.
//
// Logs at Warn level on every block so a misconfigured FE that surfaces
// the start-impersonation button to a support_agent is visible in
// operator logs. Returns 403 with JSON:API code AdminRoleRequiredCode so
// the FE can render specific copy ("ask a platform admin to do this")
// rather than a generic "forbidden" toast.
func RequirePlatformAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := appctx.BackofficeUserFromContext(r.Context())
		if user == nil {
			// RequireBackofficeAuth should have populated this. Reaching
			// here means the middleware chain is wired wrong — fail
			// closed with 401 rather than 403 so the operator notices.
			slog.Warn("RequirePlatformAdmin: no back-office user in context — middleware chain misconfigured",
				"path", r.URL.Path)
			_ = unauthorizedError(w, r, ErrMissingUserContext)
			return
		}
		if user.Role != models.BackofficeRolePlatformAdmin {
			slog.Warn("RequirePlatformAdmin: access denied",
				"admin_id", user.ID,
				"role", string(user.Role),
				"path", r.URL.Path,
			)
			_ = codedForbiddenError(w, r, ErrPlatformAdminRequired, AdminRoleRequiredCode)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireBackofficeAuthOrImpersonating gates a route subtree on EITHER
// a back-office identity (the operator who is not currently
// impersonating, or who has already ended the session) OR a request
// running inside an active impersonation session (the access token
// carries `imp=true`).
//
// The widened gate exists for GET /admin/impersonation/current: the FE
// banner needs to read its own state from inside the impersonated
// session, whose access token is a tenant JWT and so cannot satisfy
// RequireBackofficeAuth. The handler re-validates the impersonation
// claim itself, so this middleware only widens the gate — it does not
// weaken any handler-side check.
//
// Implementation: peek at the bearer token's claims FIRST to decide
// which branch to take, then dispatch — that way neither branch ever
// commits a wasted error response. A `aud=backoffice` token goes
// through RequireBackofficeAuth; a token with `imp=true` is validated
// as an impersonation token (signed, non-expired, target loadable +
// active) and admitted with the tenant user planted in context. A
// token that matches neither shape gets a single 401.
func RequireBackofficeAuthOrImpersonating(
	jwtSecret []byte,
	backofficeUserRegistry registry.BackofficeUserRegistry,
	userRegistry registry.UserRegistry,
	blacklist services.TokenBlacklister,
) func(http.Handler) http.Handler {
	backofficeGate := RequireBackofficeAuth(jwtSecret, backofficeUserRegistry, blacklist)
	return func(next http.Handler) http.Handler {
		gatedBackoffice := backofficeGate(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, peeked := peekTokenClaims(r, jwtSecret)
			if !peeked {
				_ = unauthorizedError(w, r, ErrMissingUserContext)
				return
			}
			if aud, _ := claims["aud"].(string); aud == backofficeTokenAudience {
				gatedBackoffice.ServeHTTP(w, r)
				return
			}
			if !claimsAreImpersonation(claims) {
				_ = unauthorizedError(w, r, ErrMissingUserContext)
				return
			}
			serveImpersonatingRequest(w, r, claims, userRegistry, blacklist, next)
		})
	}
}

// peekTokenClaims returns the claims of the request's bearer token
// without enforcing any plane-specific authorization. Used by the
// OR-gate to dispatch to the right downstream validator. Returns
// (nil, false) for missing/malformed headers and for forged signatures
// — both cases the caller maps to 401 directly.
func peekTokenClaims(r *http.Request, jwtSecret []byte) (jwt.MapClaims, bool) {
	tokenString, err := extractTokenFromRequest(r)
	if err != nil {
		return nil, false
	}
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil || token == nil || !token.Valid {
		return nil, false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, false
	}
	return claims, true
}

// serveImpersonatingRequest validates the supplied claims as an
// impersonation token's claims (signature already verified by the
// caller), loads the target tenant user, and dispatches next with the
// user + claims planted in context. On any validation failure writes
// a 401 — the caller has already classified the token as an
// impersonation attempt by the time it reaches here.
func serveImpersonatingRequest(
	w http.ResponseWriter,
	r *http.Request,
	claims jwt.MapClaims,
	userRegistry registry.UserRegistry,
	blacklist services.TokenBlacklister,
	next http.Handler,
) {
	if blacklist != nil {
		if err := checkTokenBlacklist(r.Context(), claims, blacklist); err != nil {
			_ = unauthorizedError(w, r, ErrMissingUserContext)
			return
		}
	}
	userID, _ := claims["user_id"].(string)
	if userID == "" {
		_ = unauthorizedError(w, r, ErrMissingUserContext)
		return
	}
	user, err := userRegistry.Get(r.Context(), userID)
	if err != nil || !user.IsActive {
		_ = unauthorizedError(w, r, ErrMissingUserContext)
		return
	}
	ctx := appctx.WithUser(r.Context(), user)
	ctx = appctx.WithJWTClaims(ctx, claims)
	next.ServeHTTP(w, r.WithContext(ctx))
}

// checkBackofficeUserBlacklistIat enforces that the token's iat is
// after the user's blacklist timestamp. Mirrors checkUserBlacklistIat.
func checkBackofficeUserBlacklistIat(claims jwt.MapClaims, since time.Time) error {
	iat, hasIat := claims["iat"].(float64)
	if !hasIat {
		return fmt.Errorf("backoffice session has been revoked")
	}
	sec, frac := math.Modf(iat)
	iatTime := time.Unix(int64(sec), int64(frac*1e9))
	if iatTime.Before(since) {
		return fmt.Errorf("backoffice session has been revoked")
	}
	return nil
}
