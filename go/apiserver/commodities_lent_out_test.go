package apiserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/checkers"
	"github.com/denisvmedia/inventario/models"
)

// TestCommoditiesList_LentOutFilter pins the wiring of the `lent_out` query
// param: presence (true|false) flips it on, and the apiserver pre-resolves
// the open-loan ID set from CommodityLoanRegistry before passing the
// filter down to the commodity registry. Closed (returned) loans MUST NOT
// flag the commodity as lent — the predicate is `returned_at IS NULL`.
func TestCommoditiesList_LentOutFilter(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	regSet := getRegistrySetFromParams(params, testUser)

	commodities := must.Must(regSet.CommodityRegistry.List(context.Background()))
	c.Assert(commodities, qt.HasLen, 2, qt.Commentf("seed expectation"))

	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
	loanReg := must.Must(params.FactorySet.CommodityLoanRegistryFactory.CreateUserRegistry(ctx))

	// Commodity 1 → open loan; Commodity 2 → returned loan (so it must
	// NOT show up under lent_out=true).
	openLoan := must.Must(loanReg.Create(ctx, models.CommodityLoan{
		CommodityID:  commodities[0].ID,
		BorrowerName: "Alice",
		LentAt:       models.Date("2026-05-01"),
	}))
	c.Assert(openLoan.IsOpen(), qt.IsTrue)

	returnedAt := models.Date("2026-05-10")
	must.Must(loanReg.Create(ctx, models.CommodityLoan{
		CommodityID:  commodities[1].ID,
		BorrowerName: "Bob",
		LentAt:       models.Date("2026-04-25"),
		ReturnedAt:   &returnedAt,
	}))

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)

	doGet := func(query string) []byte {
		c.Helper()
		req := must.Must(http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+"/commodities?"+query, nil))
		addTestUserAuthHeader(req, testUser.ID)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", rr.Body.String()))
		return rr.Body.Bytes()
	}

	c.Run("lent_out=true returns only the open-loan commodity", func(c *qt.C) {
		body := doGet("lent_out=true")
		c.Check(body, checkers.JSONPathMatches("$.data", qt.HasLen), 1)
		c.Check(body, checkers.JSONPathEquals("$.data[0].id"), commodities[0].ID)
	})

	c.Run("lent_out=false hides the open-loan commodity", func(c *qt.C) {
		body := doGet("lent_out=false")
		c.Check(body, checkers.JSONPathMatches("$.data", qt.HasLen), 1)
		c.Check(body, checkers.JSONPathEquals("$.data[0].id"), commodities[1].ID)
	})

	c.Run("no lent_out param returns both", func(c *qt.C) {
		body := doGet("")
		c.Check(body, checkers.JSONPathMatches("$.data", qt.HasLen), 2)
	})
}
