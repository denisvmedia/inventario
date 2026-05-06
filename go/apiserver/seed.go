package apiserver

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/debug/seeddata"
	"github.com/denisvmedia/inventario/registry"
)

type seedAPI struct {
	factorySet *registry.FactorySet
}

// SeedRequest represents the optional request body for seeding
type SeedRequest struct {
	UserEmail  string `json:"user_email,omitempty"`
	TenantSlug string `json:"tenant_slug,omitempty"`
}

// SeedResponse is the JSON shape returned by POST /seed. The
// AlreadySeeded flag lets callers (and tests) distinguish a first-seed
// run from an idempotent no-op without parsing the human-readable message.
type SeedResponse struct {
	Status        string `json:"status"`
	Message       string `json:"message"`
	AlreadySeeded bool   `json:"already_seeded"`
}

// seedDatabase seeds the database with example data.
// @Summary Seed database
// @Description Seed the database with example data. Optionally specify user_email and tenant_slug in request body.
// @Tags admin
// @Accept json
// @Produce json
// @Param body body SeedRequest false "Seed options (optional)"
// @Success 200 {object} map[string]string "OK"
// @Router /seed [post].
func (api *seedAPI) seedDatabase(w http.ResponseWriter, r *http.Request) {
	// Log request details
	slog.Info("=== SEED ENDPOINT CALLED ===",
		"method", r.Method,
		"content_type", r.Header.Get("Content-Type"),
		"content_length", r.ContentLength,
	)

	// Try to parse JSON body for optional parameters
	var req SeedRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil && r.ContentLength > 0 {
		// Only error if body was provided but failed to parse
		slog.Error("Failed to decode JSON body", "error", err, "content_length", r.ContentLength)
		badRequest(w, r, err)
		return
	}

	slog.Info("=== PARSED SEED REQUEST ===",
		"user_email", req.UserEmail,
		"tenant_slug", req.TenantSlug,
		"user_email_len", len(req.UserEmail),
		"tenant_slug_len", len(req.TenantSlug),
	)

	opts := seeddata.SeedOptions{
		UserEmail:  req.UserEmail,
		TenantSlug: req.TenantSlug,
	}

	alreadySeeded, err := seeddata.SeedData(api.factorySet, opts)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	message := "Database seeded successfully"
	if alreadySeeded {
		message = "Database already seeded"
	}
	render.JSON(w, r, SeedResponse{
		Status:        "success",
		Message:       message,
		AlreadySeeded: alreadySeeded,
	})
}

// Seed returns a handler for seeding the database.
func Seed(registrySet *registry.FactorySet) func(r chi.Router) {
	api := &seedAPI{
		factorySet: registrySet,
	}

	return func(r chi.Router) {
		r.Post("/", api.seedDatabase) // POST /seed
	}
}
