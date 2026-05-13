package apiserver

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// GroupPlan mounts GET /g/{groupSlug}/plan. The route lives on the
// group-scoped tree so the user-scoped RegistrySet on context is
// already filtered to the active tenant + group via RLS; the handler
// resolves the tenant's `plan_id` to the in-code `models.Plan`
// definition and aggregates per-group usage in the same shape the FE
// Plan & quota card consumes (issue #1389; unblocks #1537 item 1).
//
// Plans live in code (not the DB) in v1 — see go/models/plan.go for
// the rationale. The handler degrades to the `unlimited` plan if
// tenant.PlanID is empty or unknown (PlanByID falls back); this keeps
// the card renderable even when the tenant row was created before this
// migration ran.
func GroupPlan() func(r chi.Router) {
	return func(r chi.Router) {
		r.Get("/", handleGroupPlan)
	}
}

// handleGroupPlan returns the active plan + per-group usage.
// @Summary Get the active plan + per-group usage
// @Description Tenant plan (caps + gates) + current group usage (items, locations, storage). Plan resolved from tenants.plan_id; unknown ids degrade to unlimited. Powers the GroupSettings Plan card (#1389 / #1537).
// @Tags groups
// @Produce json
// @Param groupSlug path string true "Group slug"
// @Success 200 {object} models.GroupPlanResult "OK"
// @Failure 401 {string} string "Unauthorized"
// @Failure 500 {string} string "Internal Server Error"
// @Router /g/{groupSlug}/plan [get].
func handleGroupPlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	registrySet := RegistrySetFromContext(ctx)
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	user := appctx.UserFromContext(ctx)
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// TenantRegistry on the user-scoped Set is the same non-RLS instance
	// as `FactorySet.TenantRegistry` (it has to be — every request needs
	// to read its own tenant row to establish context, and an RLS-filtered
	// TenantRegistry would chicken-and-egg). Reading by the request user's
	// TenantID gives us exactly the row we want regardless.
	tenant, err := registrySet.TenantRegistry.Get(ctx, user.TenantID)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	plan := models.PlanByID(tenant.PlanID)

	usage, err := computeGroupUsage(ctx, registrySet)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	render.JSON(w, r, models.GroupPlanResult{
		Plan:  plan,
		Usage: usage,
	})
}

// computeGroupUsage aggregates items / locations / storage for the
// current group. Counts route through the user-scoped registries on
// the request's RegistrySet, so postgres RLS already restricts every
// query to the active (tenant, group) pair — no explicit IDs need to
// be threaded through.
func computeGroupUsage(ctx context.Context, registrySet *registry.Set) (models.PlanUsage, error) {
	items, err := registrySet.CommodityRegistry.Count(ctx)
	if err != nil {
		return models.PlanUsage{}, err
	}
	locations, err := registrySet.LocationRegistry.Count(ctx)
	if err != nil {
		return models.PlanUsage{}, err
	}
	breakdown, err := registrySet.FileRegistry.SumSizeBreakdown(ctx)
	if err != nil {
		return models.PlanUsage{}, err
	}
	return models.PlanUsage{
		Items:        items,
		Locations:    locations,
		StorageBytes: breakdown.Total(),
	}, nil
}
