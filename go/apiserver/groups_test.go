package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// groupUpdateEnv wires up a minimal /groups router so the PATCH /{groupID}
// handler can be driven from tests. It mimics the production chain but
// without the HTTP-level auth — the router stamps the user and group onto
// the request context directly, which is the part PATCH /groups actually
// cares about.
type groupUpdateEnv struct {
	router     http.Handler
	factorySet *registry.FactorySet
	user       *models.User
	group      *models.LocationGroup
}

func newGroupUpdateEnv(t *testing.T, groupCurrency models.Currency) groupUpdateEnv {
	t.Helper()

	factorySet := memory.NewFactorySet()

	tenant := must.Must(factorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "Test Tenant",
		Slug:   "test-tenant",
		Status: models.TenantStatusActive,
	}))

	// Each test spins its own factorySet, so no cross-test email collision
	// to worry about. Keep the email deterministic + noise-free in logs.
	userTemplate := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Email:               "admin@test.local",
		Name:                "Test Admin",
		IsActive:            true,
	}
	must.Assert(userTemplate.SetPassword("testpassword123"))
	user := must.Must(factorySet.UserRegistry.Create(context.Background(), userTemplate))

	slug := must.Must(models.GenerateGroupSlug())
	group := must.Must(factorySet.LocationGroupRegistry.Create(context.Background(), models.LocationGroup{
		TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: user.TenantID},
		Slug:               slug,
		Name:               "Test Group",
		Status:             models.LocationGroupStatusActive,
		CreatedBy:          user.ID,
		MainCurrency:       groupCurrency,
	}))
	must.Must(factorySet.GroupMembershipRegistry.Create(context.Background(), models.GroupMembership{
		TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: user.TenantID},
		GroupID:            group.ID,
		MemberUserID:       user.ID,
		Role:               models.GroupRoleAdmin,
	}))

	groupService := services.NewGroupService(
		factorySet.LocationGroupRegistry,
		factorySet.GroupMembershipRegistry,
		factorySet.GroupInviteRegistry,
	)

	// stampCtx injects both user and group onto the request so handlers/
	// middlewares that read them off context (RegistrySetMiddleware, the
	// PATCH /groups handler) find the wiring they expect. Production
	// routes build this via JWT + GroupSlugResolverMiddleware.
	stampCtx := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := appctx.WithUser(r.Context(), user)
			ctx = appctx.WithGroup(ctx, group)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(stampCtx)
	r.Use(apiserver.RegistrySetMiddleware(factorySet))
	r.Route("/groups", apiserver.Groups(apiserver.Params{FactorySet: factorySet}, groupService))

	return groupUpdateEnv{
		router:     r,
		factorySet: factorySet,
		user:       user,
		group:      group,
	}
}

func patchGroup(t *testing.T, env groupUpdateEnv, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()

	body := map[string]any{
		"data": map[string]any{
			"id":         env.group.ID,
			"type":       "groups",
			"attributes": payload,
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal patch body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/groups/"+env.group.ID, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	return w
}

func TestGroupsAPI_UpdateMainCurrency_UsesDefaultRate(t *testing.T) {
	c := qt.New(t)

	env := newGroupUpdateEnv(t, models.Currency("USD"))

	// Create location/area/commodity priced in USD so the rename to EUR
	// reprices the commodity using the built-in USD_EUR rate (0.85).
	ctx := appctx.WithGroup(appctx.WithUser(context.Background(), env.user), env.group)
	registrySet := must.Must(env.factorySet.CreateUserRegistrySet(ctx))

	location := must.Must(registrySet.LocationRegistry.Create(ctx, models.Location{
		Name: "Test Location", Address: "123 Test",
	}))
	area := must.Must(registrySet.AreaRegistry.Create(ctx, models.Area{
		Name: "Test Area", LocationID: location.ID,
	}))
	commodity := must.Must(registrySet.CommodityRegistry.Create(ctx, models.Commodity{
		Name:                  "Laptop",
		ShortName:             "LTP",
		Type:                  models.CommodityTypeElectronics,
		AreaID:                area.ID,
		Count:                 1,
		OriginalPrice:         decimal.RequireFromString("100"),
		OriginalPriceCurrency: models.Currency("USD"),
		CurrentPrice:          decimal.RequireFromString("60"),
		Status:                models.CommodityStatusInUse,
	}))

	resp := patchGroup(t, env, map[string]any{
		"name":          env.group.Name,
		"main_currency": "EUR",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusOK)

	var out jsonapi.LocationGroupResponse
	c.Assert(json.Unmarshal(resp.Body.Bytes(), &out), qt.IsNil)
	c.Assert(out.Data.Attributes.MainCurrency, qt.Equals, models.Currency("EUR"))

	updatedCommodity := must.Must(registrySet.CommodityRegistry.Get(ctx, commodity.ID))
	c.Assert(updatedCommodity.OriginalPriceCurrency, qt.Equals, models.Currency("EUR"))
	c.Assert(updatedCommodity.OriginalPrice.Equal(decimal.RequireFromString("85")), qt.IsTrue,
		qt.Commentf("expected reconverted original price 85, got %s", updatedCommodity.OriginalPrice))
	c.Assert(updatedCommodity.CurrentPrice.Equal(decimal.RequireFromString("51")), qt.IsTrue,
		qt.Commentf("expected reconverted current price 51, got %s", updatedCommodity.CurrentPrice))
}

func TestGroupsAPI_UpdateMainCurrency_UsesProvidedExchangeRate(t *testing.T) {
	c := qt.New(t)

	env := newGroupUpdateEnv(t, models.Currency("USD"))

	ctx := appctx.WithGroup(appctx.WithUser(context.Background(), env.user), env.group)
	registrySet := must.Must(env.factorySet.CreateUserRegistrySet(ctx))

	location := must.Must(registrySet.LocationRegistry.Create(ctx, models.Location{
		Name: "Test Location", Address: "123 Test",
	}))
	area := must.Must(registrySet.AreaRegistry.Create(ctx, models.Area{
		Name: "Test Area", LocationID: location.ID,
	}))
	commodity := must.Must(registrySet.CommodityRegistry.Create(ctx, models.Commodity{
		Name:                  "Camera",
		ShortName:             "CAM",
		Type:                  models.CommodityTypeElectronics,
		AreaID:                area.ID,
		Count:                 1,
		OriginalPrice:         decimal.RequireFromString("100"),
		OriginalPriceCurrency: models.Currency("USD"),
		CurrentPrice:          decimal.RequireFromString("40"),
		Status:                models.CommodityStatusInUse,
	}))

	resp := patchGroup(t, env, map[string]any{
		"name":          env.group.Name,
		"main_currency": "CAD",
		"exchange_rate": "1.25",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", resp.Body.String()))

	updatedCommodity := must.Must(registrySet.CommodityRegistry.Get(ctx, commodity.ID))
	c.Assert(updatedCommodity.OriginalPriceCurrency, qt.Equals, models.Currency("CAD"))
	c.Assert(updatedCommodity.OriginalPrice.Equal(decimal.RequireFromString("125")), qt.IsTrue,
		qt.Commentf("expected reconverted original price 125, got %s", updatedCommodity.OriginalPrice))
	c.Assert(updatedCommodity.CurrentPrice.Equal(decimal.RequireFromString("50")), qt.IsTrue,
		qt.Commentf("expected reconverted current price 50, got %s", updatedCommodity.CurrentPrice))
}

func TestGroupsAPI_UpdateMainCurrency_InvalidCurrencyReturnsBadRequest(t *testing.T) {
	c := qt.New(t)

	env := newGroupUpdateEnv(t, models.Currency("USD"))

	resp := patchGroup(t, env, map[string]any{
		"name":          env.group.Name,
		"main_currency": "NOPE",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusBadRequest)

	// Group record is untouched on rejection.
	current := must.Must(env.factorySet.LocationGroupRegistry.Get(context.Background(), env.group.ID))
	c.Assert(current.MainCurrency, qt.Equals, models.Currency("USD"))
}

func TestGroupsAPI_UpdateMainCurrency_SameCurrencyLeavesCommodityUntouched(t *testing.T) {
	c := qt.New(t)

	env := newGroupUpdateEnv(t, models.Currency("USD"))

	ctx := appctx.WithGroup(appctx.WithUser(context.Background(), env.user), env.group)
	registrySet := must.Must(env.factorySet.CreateUserRegistrySet(ctx))

	location := must.Must(registrySet.LocationRegistry.Create(ctx, models.Location{
		Name: "Test Location", Address: "123 Test",
	}))
	area := must.Must(registrySet.AreaRegistry.Create(ctx, models.Area{
		Name: "Test Area", LocationID: location.ID,
	}))
	commodity := must.Must(registrySet.CommodityRegistry.Create(ctx, models.Commodity{
		Name:                  "Book",
		ShortName:             "BK",
		Type:                  models.CommodityTypeOther,
		AreaID:                area.ID,
		Count:                 1,
		OriginalPrice:         decimal.RequireFromString("20"),
		OriginalPriceCurrency: models.Currency("USD"),
		CurrentPrice:          decimal.RequireFromString("15"),
		Status:                models.CommodityStatusInUse,
	}))

	resp := patchGroup(t, env, map[string]any{
		"name":          env.group.Name,
		"main_currency": "USD",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusOK)

	refreshed := must.Must(registrySet.CommodityRegistry.Get(ctx, commodity.ID))
	c.Assert(refreshed.OriginalPrice.Equal(decimal.RequireFromString("20")), qt.IsTrue)
	c.Assert(refreshed.CurrentPrice.Equal(decimal.RequireFromString("15")), qt.IsTrue)
	c.Assert(refreshed.OriginalPriceCurrency, qt.Equals, models.Currency("USD"))
}
