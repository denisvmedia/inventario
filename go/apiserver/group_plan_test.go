package apiserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/models"
)

// GET /g/{groupSlug}/plan reads the tenant's plan only when the deployment
// has plans turned on (#1389). With them off it reports the unlimited plan,
// so a tenant whose plan_id names a capped plan shows no caps, and the
// usage part is the same either way.
func TestGroupPlan(t *testing.T) {
	cases := []struct {
		name         string
		plansEnabled bool
		wantPlanID   string
	}{
		{name: "plans off reports the unlimited plan", plansEnabled: false, wantPlanID: models.PlanUnlimited.ID},
		{name: "plans on reports the tenant's plan", plansEnabled: true, wantPlanID: models.PlanFree.ID},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			params, testUser, testGroup := newParams()
			params.PlansEnabled = tc.plansEnabled

			tenant := must.Must(params.FactorySet.TenantRegistry.Get(context.Background(), testUser.TenantID))
			tenant.PlanID = models.PlanFree.ID
			must.Must(params.FactorySet.TenantRegistry.Update(context.Background(), *tenant))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/g/"+testGroup.Slug+"/plan", nil)
			addTestUserAuthHeader(req, testUser.ID)
			rr := httptest.NewRecorder()
			apiserver.APIServer(params, &mockRestoreWorker{}).ServeHTTP(rr, req)

			c.Assert(rr.Code, qt.Equals, http.StatusOK)
			body := rr.Body.Bytes()
			c.Assert(body, checkers.JSONPathEquals("$.plan.id"), tc.wantPlanID)
			if tc.plansEnabled {
				c.Assert(body, checkers.JSONPathEquals("$.plan.max_items"), float64(*models.PlanFree.MaxItems))
			} else {
				c.Assert(body, checkers.JSONPathEquals("$.plan.max_items"), nil)
				c.Assert(body, checkers.JSONPathEquals("$.plan.max_storage_bytes"), nil)
			}
			// newParams seeds two locations and two items in the group.
			c.Assert(body, checkers.JSONPathEquals("$.usage.items"), float64(2))
			c.Assert(body, checkers.JSONPathEquals("$.usage.locations"), float64(2))
		})
	}
}
