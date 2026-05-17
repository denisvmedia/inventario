package apiserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// AdminPingResponse is the body returned by GET /admin/_ping. It is
// intentionally tiny — the endpoint exists so the FE (and the swagger
// route-coverage test) has something to hit while the rest of the
// /api/v1/admin/* surface is being built out under the #1744 umbrella.
type AdminPingResponse struct {
	Ok        bool      `json:"ok"`
	Timestamp time.Time `json:"timestamp"`
}

// adminPing is the placeholder handler behind RequireSystemAdmin.
// Returns 200 with a simple JSON body so the FE can detect "I have
// system-admin" without needing a richer endpoint until later
// admin issues land.
// @Summary System-admin ping
// @Description Returns 200 when the caller has system-admin privileges. Probe endpoint for the admin surface (#1745).
// @Tags admin
// @Produce json
// @Success 200 {object} AdminPingResponse "OK"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin required"
// @Router /admin/_ping [get]
func adminPing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(AdminPingResponse{
		Ok:        true,
		Timestamp: time.Now().UTC(),
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// Admin returns the router configurator for /api/v1/admin/*. Mounted
// from apiserver.go behind the standard userMiddlewares (JWT + RLS +
// CSRF) and the RequireSystemAdmin gate. Later admin issues (#1750,
// etc.) hang their endpoints off the same chi.Router this closure
// receives.
func Admin() func(r chi.Router) {
	return func(r chi.Router) {
		// RequireSystemAdmin runs as the first per-subtree middleware so
		// every handler below it can assume the caller is a system admin.
		r.Use(RequireSystemAdmin)
		r.Get("/_ping", adminPing)
	}
}
