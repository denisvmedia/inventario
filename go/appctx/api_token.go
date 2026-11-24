package appctx

import (
	"context"

	"go.5x5.cz/inventario/models"
)

// apiTokenCtxKey holds the personal API token that authenticated the request.
// Only the token branch of apiserver's bearer authentication sets it, so its
// presence is the one signal downstream code uses to tell a token request from
// a browser session: CSRF skips on it, the group resolver enforces the token's
// group restriction from it, and the audit service stamps rows with its id.
const apiTokenCtxKey contextKey = "apiToken"

// WithAPIToken attaches the authenticating token. A nil token leaves the
// context unchanged.
func WithAPIToken(ctx context.Context, token *models.APIToken) context.Context {
	if token == nil {
		return ctx
	}
	return context.WithValue(ctx, apiTokenCtxKey, token)
}

// APITokenFromContext returns the token that authenticated the request, or nil
// when the request was authenticated some other way (or not at all).
func APITokenFromContext(ctx context.Context) *models.APIToken {
	token, ok := ctx.Value(apiTokenCtxKey).(*models.APIToken)
	if !ok {
		return nil
	}
	return token
}
