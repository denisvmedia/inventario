package appctx

import (
	"context"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// jwtClaimsCtxKey holds the parsed JWT claims for the current request.
	// Plumbed by apiserver.JWTMiddleware after token validation so downstream
	// helpers (e.g. services.AuditService.LogAdmin) can read claims like
	// `imp` / `impersonated_by` without having to re-parse the bearer token.
	jwtClaimsCtxKey contextKey = "jwtClaims"
)

// JWTClaimsFromContext returns the JWT MapClaims previously set by
// WithJWTClaims, or nil if no claims have been attached. Callers MUST
// handle nil — non-HTTP code paths (CLI, workers) will never have claims
// in context.
func JWTClaimsFromContext(ctx context.Context) jwt.MapClaims {
	claims, ok := ctx.Value(jwtClaimsCtxKey).(jwt.MapClaims)
	if !ok {
		return nil
	}
	return claims
}

// WithJWTClaims attaches a set of validated JWT claims to the context so
// downstream helpers can read them. Set after token validation in
// apiserver.JWTMiddleware; intentionally a no-op when claims is nil so
// tests that bypass the middleware (and just stamp a user via WithUser)
// don't end up with a misleading "claims present" signal.
func WithJWTClaims(ctx context.Context, claims jwt.MapClaims) context.Context {
	if claims == nil {
		return ctx
	}
	return context.WithValue(ctx, jwtClaimsCtxKey, claims)
}

// ImpersonatorID returns the back-office operator acting through an
// impersonation session, read from the `impersonator_id` claim (or the
// earlier `impersonated_by`) when the `imp` claim is true, or "" when the
// request is not impersonated. It is the value audit rows record as
// impersonated_by.
func ImpersonatorID(ctx context.Context) string {
	claims := JWTClaimsFromContext(ctx)
	if claims == nil {
		return ""
	}
	if imp, _ := claims["imp"].(bool); !imp {
		return ""
	}
	if by, ok := claims["impersonator_id"].(string); ok && by != "" {
		return by
	}
	if by, ok := claims["impersonated_by"].(string); ok && by != "" {
		return by
	}
	return ""
}
