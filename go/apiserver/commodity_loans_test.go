package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// countingCommodityRegistry decorates a registry.CommodityRegistry with
// per-instance call counters. Pointer fields keep the same counters
// shared across every factory-produced registry within a test run, so
// the handler test can assert "exactly one batched fetch, zero
// per-loan Gets" across the lifetime of a single HTTP request.
type countingCommodityRegistry struct {
	registry.CommodityRegistry
	getCalls     *atomic.Int64
	getManyCalls *atomic.Int64
}

func (r *countingCommodityRegistry) Get(ctx context.Context, id string) (*models.Commodity, error) {
	r.getCalls.Add(1)
	return r.CommodityRegistry.Get(ctx, id)
}

func (r *countingCommodityRegistry) GetMany(ctx context.Context, ids []string) ([]*models.Commodity, error) {
	r.getManyCalls.Add(1)
	return r.CommodityRegistry.GetMany(ctx, ids)
}

// countingCommodityRegistryFactory wraps a registry.CommodityRegistryFactory
// so every CreateUserRegistry / CreateServiceRegistry call returns a
// counter-decorated registry. The factory itself is stateless — the
// counters live on the factory struct, not the per-request registry,
// because a single HTTP request can ask the factory for multiple
// registries (one per middleware-injected ctx scope) and the test
// wants to see the *total* fetch traffic, not the per-instance one.
type countingCommodityRegistryFactory struct {
	inner        registry.CommodityRegistryFactory
	getCalls     *atomic.Int64
	getManyCalls *atomic.Int64
}

func (f *countingCommodityRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.CommodityRegistry, error) {
	inner, err := f.inner.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}
	return &countingCommodityRegistry{
		CommodityRegistry: inner,
		getCalls:          f.getCalls,
		getManyCalls:      f.getManyCalls,
	}, nil
}

func (f *countingCommodityRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.CommodityRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *countingCommodityRegistryFactory) CreateServiceRegistry() registry.CommodityRegistry {
	return &countingCommodityRegistry{
		CommodityRegistry: f.inner.CreateServiceRegistry(),
		getCalls:          f.getCalls,
		getManyCalls:      f.getManyCalls,
	}
}

// TestListGroupLoans_BatchedCommodityFetch pins the fix for issue #1512:
// however many loans the page returns, the handler resolves their parent
// commodities in a single GetMany round-trip — never a per-row Get loop.
// Counters are injected via a wrapping CommodityRegistryFactory so the
// assertion covers whatever path the handler takes (direct registry use,
// middleware-injected scopes, future internal callers) without coupling
// to specific call sites.
func TestListGroupLoans_BatchedCommodityFetch(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()

	// Wrap the existing CommodityRegistryFactory with a counter. Reaching
	// past newParams' helper into params.FactorySet is intentional —
	// using newParams keeps the seed data identical to the rest of the
	// apiserver tests, and the wrap-after pattern means future newParams
	// changes don't require touching this test.
	getCalls := &atomic.Int64{}
	getManyCalls := &atomic.Int64{}
	params.FactorySet.CommodityRegistryFactory = &countingCommodityRegistryFactory{
		inner:        params.FactorySet.CommodityRegistryFactory,
		getCalls:     getCalls,
		getManyCalls: getManyCalls,
	}

	registrySet := getRegistrySetFromParams(params, testUser)
	areas := must.Must(registrySet.AreaRegistry.List(context.Background()))
	c.Assert(areas, qt.Not(qt.HasLen), 0, qt.Commentf("seed data did not produce any areas"))
	area := areas[0]

	// Seed commodities with Count=1: loans are per-instance and the
	// per-instance guard rejects loan-create on Count>1 (see
	// EnsureCommodityTrackable in services/commodity_quantity_guard.go).
	// The default newParams seed has bundle commodities (Count=10/5), so
	// we add our own single-instance rows. The N+1 fix is independent of
	// commodity shape; the point of the test is the page-fetch traffic
	// against /loans.
	mkCommodity := func(name string) *models.Commodity {
		return must.Must(registrySet.CommodityRegistry.Create(context.Background(), models.Commodity{
			Name:                  name,
			ShortName:             name,
			AreaID:                new(area.ID),
			Status:                models.CommodityStatusInUse,
			Type:                  models.CommodityTypeElectronics,
			Count:                 1,
			OriginalPrice:         must.Must(decimal.NewFromString("100.00")),
			OriginalPriceCurrency: models.Currency("USD"),
		}))
	}
	// Three commodities so loan creates can succeed independently. The
	// page will see 3+ loans for ≤3 distinct commodities — the dedup
	// path in uniqueCommodityIDsForLoans matters, GetMany should still
	// be one call.
	commodities := []*models.Commodity{
		mkCommodity("loanable-1"),
		mkCommodity("loanable-2"),
		mkCommodity("loanable-3"),
	}

	loanBodies := []struct {
		commodityIdx int
		borrower     string
		lentAt       string
	}{
		{0, "Alice", "2026-05-01"},
		{1, "Bob", "2026-05-02"},
		{2, "Charlie", "2026-05-03"},
	}

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)

	openedLoans := 0
	for _, lb := range loanBodies {
		if lb.commodityIdx >= len(commodities) {
			continue
		}
		commodity := commodities[lb.commodityIdx]
		body := `{"data":{"type":"commodity_loans","attributes":{"borrower_name":"` + lb.borrower +
			`","lent_at":"` + lb.lentAt + `"}}}`
		req := must.Must(http.NewRequest("POST",
			"/api/v1/g/"+testGroup.Slug+"/commodities/"+commodity.ID+"/loans",
			bytes.NewBufferString(body)))
		req.Header.Set("Content-Type", "application/vnd.api+json")
		addTestUserAuthHeader(req, testUser.ID)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		switch rr.Code {
		case http.StatusCreated:
			openedLoans++
		case http.StatusConflict:
			// A second loan on a commodity that already has an open
			// loan is rejected — fine, we just need a few open loans
			// across multiple commodities, not one per attempt.
		default:
			c.Fatalf("unexpected status opening loan: %d body=%s", rr.Code, rr.Body.String())
		}
	}
	c.Assert(openedLoans, qt.Not(qt.Equals), 0, qt.Commentf("expected at least one loan to open against the seed commodities"))

	// Reset the counters right before the SUT call. Loan-create handlers
	// don't fetch the commodity through CommodityRegistry today, but
	// snapshotting the counters here means a future change that adds a
	// commodity Get to the create path won't silently pollute this
	// assertion.
	getCalls.Store(0)
	getManyCalls.Store(0)

	// SUT: GET /loans — the surface this test exists to lock down.
	req := must.Must(http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+"/loans", nil))
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))

	// Sanity: response actually carries commodity refs, so a passing
	// "zero fetches" assertion below is real and not a vacuous truth
	// from an empty page.
	var listResp jsonapi.CommodityLoanListResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &listResp), qt.IsNil)
	c.Assert(listResp.Data, qt.Not(qt.HasLen), 0)
	withCommodityRef := 0
	for _, item := range listResp.Data {
		if item.Commodity != nil {
			withCommodityRef++
		}
	}
	c.Assert(withCommodityRef, qt.Not(qt.Equals), 0, qt.Commentf("response should populate commodity refs from the batched fetch"))

	// The handler must fetch commodities in a single batched call,
	// regardless of how many loans are on the page or how many
	// commodities they reference. This is the N+1 fix.
	c.Assert(getManyCalls.Load(), qt.Equals, int64(1), qt.Commentf("expected exactly one batched commodity fetch, got %d", getManyCalls.Load()))
	c.Assert(getCalls.Load(), qt.Equals, int64(0), qt.Commentf("expected no per-loan commodity Get calls, got %d", getCalls.Load()))
}
