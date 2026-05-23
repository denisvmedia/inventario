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
