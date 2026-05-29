package apiserver

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/debug/seeddata"
	"github.com/denisvmedia/inventario/registry"
)

// envSeedSystemAdminFixture is the opt-in env var that lets the seed
// provision the `sysadmin@test-org.com` fixture with the platform-wide
// is_system_admin flag (#1758). It is OFF by default: /api/v1/seed is
// unauthenticated, so minting a cross-tenant admin from it would be a
// privilege-escalation hole. Only the e2e harness sets it.
const envSeedSystemAdminFixture = "INVENTARIO_SEED_SYSTEM_ADMIN_FIXTURE"

// envSeedAllowCreateTenant is the opt-in env var that lets the seed
// create a tenant row when the request supplies a tenant_slug that
// doesn't exist (#1851). It is OFF by default for the same reason
// envSeedSystemAdminFixture is — /api/v1/seed is unauthenticated, so
// allowing arbitrary tenant creation from it would let any caller
// mint isolation boundaries in a misconfigured deployment. The e2e
// harness sets this when it needs a second tenant for the
// cross-tenant OAuth fixture.
const envSeedAllowCreateTenant = "INVENTARIO_SEED_ALLOW_CREATE_TENANT"

// envSeedAllowBlobUploads is the opt-in env var that lets the seed write
// the bundled fixture *bytes* (photos, invoices, manuals) into the
// configured blob bucket for a non-`test-org` tenant — not just the
// metadata rows. It is OFF by default for the same reason
// envSeedAllowCreateTenant is: /api/v1/seed is unauthenticated, so
// writing real bytes for an arbitrary tenant_slug would let any caller
// spam the configured bucket. The Helm chart sets it (only on the
// init-data Job's temporary server) for the demo overlay so the
// evaluation deployment looks lived-in.
const envSeedAllowBlobUploads = "INVENTARIO_SEED_ALLOW_BLOB_UPLOADS"

type seedAPI struct {
	factorySet     *registry.FactorySet
	uploadLocation string
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
// @Success 200 {object} SeedResponse "OK"
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
		UserEmail:      req.UserEmail,
		TenantSlug:     req.TenantSlug,
		UploadLocation: api.uploadLocation,
		// Opt-in only — see envSeedSystemAdminFixture. Read server-side
		// from the environment, never from the (attacker-controllable)
		// request body.
		SeedSystemAdmin: os.Getenv(envSeedSystemAdminFixture) == "true",
		// Opt-in only — see envSeedAllowCreateTenant. Same env-gated
		// pattern as SeedSystemAdmin: never sourced from the request
		// body so a misconfigured production deployment can't be
		// coaxed into minting tenants from the public seed surface.
		CreateTenantIfMissing: os.Getenv(envSeedAllowCreateTenant) == "true",
		// Opt-in only — see envSeedAllowBlobUploads. Same env-gated
		// pattern: lets the demo overlay publish bundled fixture bytes
		// for its `default` tenant without opening the public seed
		// surface to bucket-spam from arbitrary tenant_slug values.
		AllowBlobUploads: os.Getenv(envSeedAllowBlobUploads) == "true",
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

// Seed returns a handler for seeding the database. The uploadLocation
// is plumbed through to seeddata.SeedData so bundled file fixtures
// (photos, invoices, manuals) get written into the same blob bucket
// the live upload path uses; pass "" to skip blob writes (the seed
// will still create metadata-only file rows).
func Seed(registrySet *registry.FactorySet, uploadLocation string) func(r chi.Router) {
	api := &seedAPI{
		factorySet:     registrySet,
		uploadLocation: uploadLocation,
	}

	return func(r chi.Router) {
		r.Post("/", api.seedDatabase) // POST /seed
	}
}
