package apiserver

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/go-extras/errx"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/internal/errormarshal"
	"go.5x5.cz/inventario/internal/metrics"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// JSON:API error codes a token request can receive. Scripts branch on these,
// so they are part of the documented contract.
const (
	apiTokenInvalidCode            = "api_token.invalid"               // #nosec G101 -- an error code, not a credential
	apiTokenAmbiguousCode          = "api_token.ambiguous_credentials" // #nosec G101 -- an error code, not a credential
	apiTokenReadOnlyCode           = "api_token.read_only"             // #nosec G101 -- an error code, not a credential
	apiTokenEndpointNotAllowedCode = "api_token.endpoint_not_allowed"  // #nosec G101 -- an error code, not a credential
	apiTokenGroupNotAllowedCode    = "api_token.group_not_allowed"     // #nosec G101 -- an error code, not a credential
	apiTokenRateLimitedCode        = "api_token.rate_limited"          // #nosec G101 -- an error code, not a credential
)

var (
	errAPITokenEndpointNotAllowed = errx.NewSentinel("API tokens are not accepted on this endpoint")
	errAPITokenReadOnly           = errx.NewSentinel("this API token is read-only")
	errAPITokenGroupNotAllowed    = errx.NewSentinel("this API token is restricted to another group")
	errAPITokenAmbiguous          = errx.NewSentinel("send exactly one Authorization header")
	errAPITokenRateLimited        = errx.NewSentinel("rate limit exceeded")
)

// APITokenAuthenticator authenticates requests that carry a personal API token
// (#2808) and applies the rules that hold for every token request: one
// credential per request, rate limits per client address and per token, the
// token's scope, and a throttled last-used record with its audit row.
//
// It does not decide where tokens are accepted. Only the routes mounted with
// SessionOrAPITokenMiddleware accept them; JWTMiddleware and RejectAPITokens
// refuse them everywhere else, so a new route is closed to tokens until it is
// deliberately opened.
type APITokenAuthenticator struct {
	service          *services.APITokenService
	audit            services.AuditLogger
	limiter          services.GlobalRateLimiter
	trustedProxyNets []*net.IPNet
}

// NewAPITokenAuthenticator wires the authenticator. limiter may be nil, which
// disables the token rate limits (tests); audit may be nil.
func NewAPITokenAuthenticator(
	service *services.APITokenService,
	audit services.AuditLogger,
	limiter services.GlobalRateLimiter,
	trustedProxyNets []*net.IPNet,
) *APITokenAuthenticator {
	return &APITokenAuthenticator{
		service:          service,
		audit:            audit,
		limiter:          limiter,
		trustedProxyNets: trustedProxyNets,
	}
}

// SessionOrAPITokenMiddleware authenticates a request by its Authorization
// header and nothing else. A bearer credential with the personal-token prefix
// is authenticated as a token; anything else takes the existing JWT access
// token path unchanged. No cookie takes part in either decision — the refresh
// cookie the browser sends along is only ever read by /auth/refresh — so a
// request that carries a cookie and a token is a token request, and a request
// that carries a cookie and a JWT is a session request with the usual CSRF
// requirement.
//
// A nil authenticator makes this exactly JWTMiddleware, which refuses tokens.
func SessionOrAPITokenMiddleware(
	jwtSecret []byte,
	userRegistry registry.UserRegistry,
	blacklist services.TokenBlacklister,
	tokens *APITokenAuthenticator,
) func(http.Handler) http.Handler {
	jwtAuth := JWTMiddleware(jwtSecret, userRegistry, blacklist)
	return func(next http.Handler) http.Handler {
		sessionNext := jwtAuth(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Two Authorization headers would let the credential that wins
			// depend on header order. Refuse rather than pick one.
			if len(r.Header.Values("Authorization")) > 1 {
				_ = codedUnauthorizedError(w, r, errAPITokenAmbiguous, apiTokenAmbiguousCode)
				return
			}
			credential, ok := parseBearerToken(r.Header.Get("Authorization"))
			if !ok || !models.LooksLikeAPIToken(credential) || tokens == nil {
				sessionNext.ServeHTTP(w, r)
				return
			}
			tokens.serve(w, r, credential, next)
		})
	}
}

func (a *APITokenAuthenticator) serve(w http.ResponseWriter, r *http.Request, credential string, next http.Handler) {
	ip := clientIPForGlobalRateLimit(r, a.trustedProxyNets)

	// The per-address budget is the same one unauthenticated requests draw
	// from, and it is checked before the token is looked up, so guessing
	// tokens costs the guesser that budget and nothing else.
	if !a.allow(w, r, ip, "ip", ip) {
		return
	}

	token, user, err := a.service.Authenticate(r.Context(), credential, TenantIDFromContext(r.Context()))
	if err != nil {
		if errors.Is(err, services.ErrAPITokenInvalid) {
			// The reason stays in the log. The response is the same for an
			// unknown, revoked or expired token and for an inactive owner.
			slog.WarnContext(r.Context(), "API token rejected",
				"reason", services.APITokenFailureReason(err),
				"ip", ip,
				"method", r.Method,
				"path", r.URL.Path,
			)
			_ = codedUnauthorizedError(w, r, services.ErrAPITokenInvalid, apiTokenInvalidCode)
			return
		}
		_ = internalServerError(w, r, err)
		return
	}

	if !token.AllowsMethod(r.Method) {
		_ = codedForbiddenError(w, r, errAPITokenReadOnly, apiTokenReadOnlyCode)
		return
	}

	// Each token also has its own budget, so a leaked token used from many
	// addresses is still bounded.
	if !a.allow(w, r, ip, "token", "api-token:"+token.ID) {
		return
	}

	ctx := appctx.WithUser(r.Context(), user)
	ctx = appctx.WithAPIToken(ctx, token)
	r = r.WithContext(ctx)

	a.recordUse(r, token, user)
	next.ServeHTTP(w, r)
}

// allow checks one rate-limit key and writes the 429 itself when the budget is
// spent. A limiter error lets the request through, the same fail-open choice
// GlobalRateLimitMiddleware makes: an outage of the limiter backend must not
// take the API down.
func (a *APITokenAuthenticator) allow(w http.ResponseWriter, r *http.Request, ip, scope, key string) bool {
	if a.limiter == nil {
		return true
	}
	res, err := a.limiter.Check(r.Context(), key)
	if err != nil {
		slog.ErrorContext(r.Context(), "API token rate limiter error", "error", err, "scope", scope)
		return true
	}
	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", res.Limit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", res.Remaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", res.ResetAt.Unix()))
	if res.Allowed {
		return true
	}
	retryAfter := max(int(time.Until(res.ResetAt).Seconds()), 0)
	w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
	metrics.RecordRateLimitRejection(metrics.RateLimitScopeGlobal)
	slog.WarnContext(r.Context(), "API token rate limit exceeded", "scope", scope, "ip", ip, "path", r.URL.Path, "retry_after_seconds", retryAfter)
	_ = codedTooManyRequestsError(w, r, errAPITokenRateLimited, apiTokenRateLimitedCode,
		map[string]any{"retry_after_seconds": retryAfter})
	return false
}

// recordUse updates last_used_at at most once per interval and writes one
// api_token_use audit row each time it does. A failure here is logged and
// never fails the request.
func (a *APITokenAuthenticator) recordUse(r *http.Request, token *models.APIToken, user *models.User) {
	touched, err := a.service.RecordUse(r.Context(), token)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to record API token use", "token_id", token.ID, "error", err)
		return
	}
	if !touched || a.audit == nil {
		return
	}
	a.audit.LogAuth(r.Context(), services.AuthEvent{
		Action:     "api_token_use",
		UserID:     new(user.ID),
		TenantID:   new(user.TenantID),
		EntityType: new(apiTokenAuditEntityType),
		EntityID:   new(token.ID),
		Success:    true,
		Request:    r,
	})
}

// RejectAPITokens refuses any request whose bearer credential is a personal
// API token. It guards the subtrees that authenticate some other way or not at
// all — /auth (login, refresh, logout, MFA, OAuth), the back-office plane and
// /admin — so a token is turned away with a clear answer there instead of
// being read as a malformed JWT or ignored.
func RejectAPITokens() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if requestCarriesAPIToken(r) {
				_ = codedForbiddenError(w, r, errAPITokenEndpointNotAllowed, apiTokenEndpointNotAllowedCode)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// apiTokenAllowlist refuses a personal API token on every route of the router
// it is installed on, except the GET routes that allowed matches. A route added
// to that router later is closed to tokens until someone adds it to the
// allowlist, so a new admin read cannot leak through by default.
//
// It must be installed with Use inside a mounted router: chi's RoutePath then
// holds the path within that router, which is what allowed is matched against.
// A request it cannot place is refused.
func apiTokenAllowlist(allowed chi.Routes) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if appctx.APITokenFromContext(r.Context()) != nil && !apiTokenRouteAllowed(allowed, r) {
				_ = codedForbiddenError(w, r, errAPITokenEndpointNotAllowed, apiTokenEndpointNotAllowedCode)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func apiTokenRouteAllowed(allowed chi.Routes, r *http.Request) bool {
	method := r.Method
	if method == http.MethodHead {
		method = http.MethodGet
	}
	if method != http.MethodGet {
		return false
	}
	rctx := chi.RouteContext(r.Context())
	if rctx == nil || rctx.RoutePath == "" {
		return false
	}
	return allowed.Match(chi.NewRouteContext(), method, rctx.RoutePath)
}

// apiTokenAllowsGroup reports whether the request may act on groupID: true
// for a session request or an unrestricted token, and for a restricted token
// only when groupID is its group.
func apiTokenAllowsGroup(r *http.Request, groupID string) bool {
	token := appctx.APITokenFromContext(r.Context())
	if token == nil || token.GroupID == nil {
		return true
	}
	return *token.GroupID == groupID
}

func renderAPITokenGroupNotAllowed(w http.ResponseWriter, r *http.Request) {
	_ = codedForbiddenError(w, r, errAPITokenGroupNotAllowed, apiTokenGroupNotAllowedCode)
}

// requestCarriesAPIToken reports whether the Authorization header holds a
// personal API token. It checks the prefix only; validity does not matter to
// a route that refuses tokens outright.
func requestCarriesAPIToken(r *http.Request) bool {
	for _, header := range r.Header.Values("Authorization") {
		if credential, ok := parseBearerToken(header); ok && models.LooksLikeAPIToken(credential) {
			return true
		}
	}
	return false
}

// codedUnauthorizedError renders a 401 with a JSON:API error code.
func codedUnauthorizedError(w http.ResponseWriter, r *http.Request, err error, code string) error {
	jsErr := jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(err),
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     "Unauthorized",
		Code:           code,
	}
	return render.Render(w, r, jsonapi.NewErrors(jsErr))
}
