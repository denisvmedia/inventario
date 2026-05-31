package apiserver

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// FeatureFlags is the public, deployment-scoped feature-flag surface
// (#1616). The FE reads it once at boot so it can hide entry points
// for features whose backend is gated off — otherwise the user sees
// the CTA, clicks it, and the request silently 404s.
//
// Public (unauthenticated) on purpose:
//   - Flags describe deployment posture, not per-user authorization.
//   - The unauthenticated /login surface also benefits (e.g. hiding
//     "Sign up" copy if registration is closed — out of scope here,
//     wired for future flags).
//   - Avoids a chicken-and-egg between bootstrap and /auth/me.
//
// Field names are stable JSON keys; the FE branches on them. Add new
// flags as additional fields rather than a generic map so swagger
// stays usefully typed and code-search finds the call sites.
type FeatureFlags struct {
	// CurrencyMigration mirrors Params.FeatureCurrencyMigration. When
	// false the /currency-migrations endpoints return a coded 404 and
	// the FE hides the wizard entry point + history sheet on the
	// group-settings page.
	CurrencyMigration bool `json:"currency_migration"`

	// MagicLinkLogin mirrors Params.MagicLinkLoginEnabled. The entry point
	// is pre-login (a "Email me a sign-in link" affordance on the Login
	// page), so it is exposed here on the public /feature-flags surface
	// rather than the auth-gated /system endpoint. When false the FE hides
	// the affordance and the /auth/magic-link routes return 404.
	MagicLinkLogin bool `json:"magic_link_login"`
}

// FeatureFlagsHandler returns a constant-shaped feature-flag payload
// derived from Params at server boot. The values do not change at
// runtime — flipping a flag requires a re-deploy (operator kill-switch
// semantics, #1604) — so we don't bother with a fetch-on-every-call
// indirection.
//
// @Summary Get deployment feature flags
// @Description Returns the deployment-scoped feature-flag state. Public (no auth) — flags describe deployment posture, not per-user authorization. Used by the FE at boot to hide entry points for features whose backend is gated off (#1616).
// @Tags system
// @Produce json
// @Success 200 {object} FeatureFlags "OK"
// @Router /feature-flags [get].
func FeatureFlagsHandler(params Params) func(r chi.Router) {
	flags := FeatureFlags{
		CurrencyMigration: params.FeatureCurrencyMigration,
		MagicLinkLogin:    params.MagicLinkLoginEnabled,
	}
	return func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			// Static payload; ignore encoder errors — there's nothing
			// useful to log if a struct of bools fails to serialize.
			_ = json.NewEncoder(w).Encode(flags)
		})
	}
}
