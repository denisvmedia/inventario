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

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// groupTestEnv wires up a minimal /groups router so the create and update
// handlers can be driven from tests. It mimics the production chain but
// without the HTTP-level auth — the router stamps the user (and optionally
// a group) onto the request context directly, which is the part the
// /groups handlers actually care about.
type groupTestEnv struct {
	router     http.Handler
	factorySet *registry.FactorySet
	user       *models.User
	group      *models.LocationGroup
}

func newGroupTestEnv(t *testing.T, groupCurrency models.Currency) groupTestEnv {
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

	var group *models.LocationGroup
	if groupCurrency != "" {
		slug := must.Must(models.GenerateGroupSlug())
		group = must.Must(factorySet.LocationGroupRegistry.Create(context.Background(), models.LocationGroup{
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
	}

	groupService := services.NewGroupService(
		factorySet.LocationGroupRegistry,
		factorySet.GroupMembershipRegistry,
		factorySet.GroupInviteRegistry,
	)

	// stampCtx injects user (and the current group, when present) onto the
	// request. Production routes build this via JWT +
	// GroupSlugResolverMiddleware; tests short-circuit because the handlers
	// only care about the context shape, not where it came from —
	// GetUserFromRequest just reads appctx.UserFromContext.
	stampCtx := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := appctx.WithUser(r.Context(), user)
			if group != nil {
				ctx = appctx.WithGroup(ctx, group)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(stampCtx)
	r.Use(apiserver.RegistrySetMiddleware(factorySet))
	r.Route("/groups", apiserver.Groups(apiserver.Params{FactorySet: factorySet}, groupService, nil))

	return groupTestEnv{
		router:     r,
		factorySet: factorySet,
		user:       user,
		group:      group,
	}
}

func postGroup(t *testing.T, env groupTestEnv, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()

	body := map[string]any{
		"data": map[string]any{
			"type":       "groups",
			"attributes": payload,
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal post body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/groups", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	return w
}

func patchGroup(t *testing.T, env groupTestEnv, payload map[string]any) *httptest.ResponseRecorder {
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

func TestGroupsAPI_CreateGroup_WithExplicitMainCurrency(t *testing.T) {
	c := qt.New(t)

	// No pre-existing group — we're testing the create path.
	env := newGroupTestEnv(t, "")

	resp := postGroup(t, env, map[string]any{
		"name":          "Brand New",
		"icon":          "🏠",
		"main_currency": "EUR",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusCreated, qt.Commentf("body: %s", resp.Body.String()))

	var out jsonapi.LocationGroupResponse
	c.Assert(json.Unmarshal(resp.Body.Bytes(), &out), qt.IsNil)
	c.Assert(out.Data.Attributes.Name, qt.Equals, "Brand New")
	c.Assert(out.Data.Attributes.MainCurrency, qt.Equals, models.Currency("EUR"))
}

func TestGroupsAPI_CreateGroup_DefaultsMainCurrencyToUSD(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, "")

	resp := postGroup(t, env, map[string]any{
		"name": "No-currency group",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusCreated, qt.Commentf("body: %s", resp.Body.String()))

	var out jsonapi.LocationGroupResponse
	c.Assert(json.Unmarshal(resp.Body.Bytes(), &out), qt.IsNil)
	c.Assert(out.Data.Attributes.MainCurrency, qt.Equals, models.Currency("USD"))
}

func TestGroupsAPI_CreateGroup_InvalidMainCurrencyReturnsBadRequest(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, "")

	resp := postGroup(t, env, map[string]any{
		"name":          "Bad currency group",
		"main_currency": "NOPE",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusBadRequest, qt.Commentf("body: %s", resp.Body.String()))

	// No group was persisted on the failed request.
	groups := must.Must(env.factorySet.LocationGroupRegistry.List(context.Background()))
	c.Assert(groups, qt.HasLen, 0)
}

func TestGroupsAPI_UpdateGroup_RejectsMainCurrencyChange(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, models.Currency("USD"))

	resp := patchGroup(t, env, map[string]any{
		"name":          env.group.Name,
		"main_currency": "EUR",
	})
	// #202 tracks the currency-migration tool. Until it lands, rejecting
	// loudly is better than silently dropping the change.
	c.Assert(resp.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body: %s", resp.Body.String()))

	current := must.Must(env.factorySet.LocationGroupRegistry.Get(context.Background(), env.group.ID))
	c.Assert(current.MainCurrency, qt.Equals, models.Currency("USD"))
}

func TestGroupsAPI_UpdateGroup_SameMainCurrencyIsNoOp(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, models.Currency("USD"))

	resp := patchGroup(t, env, map[string]any{
		"name":          "Renamed",
		"main_currency": "USD",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", resp.Body.String()))

	current := must.Must(env.factorySet.LocationGroupRegistry.Get(context.Background(), env.group.ID))
	c.Assert(current.Name, qt.Equals, "Renamed")
	c.Assert(current.MainCurrency, qt.Equals, models.Currency("USD"))
}

// Issue #1255: the icon field used to accept any string up to 10 chars,
// which let typos and nonsense slip in and render as literal text. It is
// now constrained to the curated ValidGroupIcons set (or empty).
func TestGroupsAPI_CreateGroup_RejectsIconOutsideCuratedSet(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, "")

	resp := postGroup(t, env, map[string]any{
		"name": "Bad Icon Group",
		"icon": "xyz",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body: %s", resp.Body.String()))

	groups := must.Must(env.factorySet.LocationGroupRegistry.List(context.Background()))
	c.Assert(groups, qt.HasLen, 0)
}

func TestGroupsAPI_CreateGroup_AcceptsEmptyIcon(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, "")

	resp := postGroup(t, env, map[string]any{
		"name": "No Icon Group",
		"icon": "",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusCreated, qt.Commentf("body: %s", resp.Body.String()))

	groups := must.Must(env.factorySet.LocationGroupRegistry.List(context.Background()))
	c.Assert(groups, qt.HasLen, 1)
	c.Assert(groups[0].Icon, qt.Equals, "")
}

func TestGroupsAPI_CreateGroup_AcceptsCuratedIcon(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, "")

	resp := postGroup(t, env, map[string]any{
		"name": "Valid Icon Group",
		"icon": "📦",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusCreated, qt.Commentf("body: %s", resp.Body.String()))

	groups := must.Must(env.factorySet.LocationGroupRegistry.List(context.Background()))
	c.Assert(groups, qt.HasLen, 1)
	c.Assert(groups[0].Icon, qt.Equals, "📦")
}

func TestGroupsAPI_UpdateGroup_RejectsIconOutsideCuratedSet(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, models.Currency("USD"))

	resp := patchGroup(t, env, map[string]any{
		"name": env.group.Name,
		"icon": "nope",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body: %s", resp.Body.String()))

	current := must.Must(env.factorySet.LocationGroupRegistry.Get(context.Background(), env.group.ID))
	c.Assert(current.Icon, qt.Equals, "")
}

// deleteGroup — spec #1219 §12: admin must type the group name AND their
// current password. Both checks are distinguishable (different error
// surfaces) so the frontend can render specific copy for each failure.

func deleteGroup(t *testing.T, env groupTestEnv, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal delete body: %v", err)
	}
	req := httptest.NewRequest(http.MethodDelete, "/groups/"+env.group.ID, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	return w
}

func TestGroupsAPI_DeleteGroup_HappyPath(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, models.Currency("USD"))

	resp := deleteGroup(t, env, map[string]any{
		"confirm_word": env.group.Name,
		"password":     "testpassword123",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusNoContent, qt.Commentf("body: %s", resp.Body.String()))

	current := must.Must(env.factorySet.LocationGroupRegistry.Get(context.Background(), env.group.ID))
	c.Assert(current.Status, qt.Equals, models.LocationGroupStatusPendingDeletion)
}

func TestGroupsAPI_DeleteGroup_WrongPasswordReturns422(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, models.Currency("USD"))

	resp := deleteGroup(t, env, map[string]any{
		"confirm_word": env.group.Name,
		"password":     "not-the-real-password",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body: %s", resp.Body.String()))

	// Distinguishable error — body carries the password-specific message,
	// not the confirm-word one. This is what the frontend keys off to
	// render inline per-field feedback.
	c.Assert(resp.Body.String(), qt.Contains, "password")

	// Group must not have been touched.
	current := must.Must(env.factorySet.LocationGroupRegistry.Get(context.Background(), env.group.ID))
	c.Assert(current.Status, qt.Equals, models.LocationGroupStatusActive)
}

func TestGroupsAPI_DeleteGroup_WrongConfirmWordReturns422(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, models.Currency("USD"))

	resp := deleteGroup(t, env, map[string]any{
		"confirm_word": "not-the-group-name",
		"password":     "testpassword123",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body: %s", resp.Body.String()))
	c.Assert(resp.Body.String(), qt.Contains, "confirmation")

	current := must.Must(env.factorySet.LocationGroupRegistry.Get(context.Background(), env.group.ID))
	c.Assert(current.Status, qt.Equals, models.LocationGroupStatusActive)
}

func TestGroupsAPI_DeleteGroup_MissingPasswordRejectedAtBind(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, models.Currency("USD"))

	resp := deleteGroup(t, env, map[string]any{
		"confirm_word": env.group.Name,
	})
	c.Assert(resp.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body: %s", resp.Body.String()))

	current := must.Must(env.factorySet.LocationGroupRegistry.Get(context.Background(), env.group.ID))
	c.Assert(current.Status, qt.Equals, models.LocationGroupStatusActive)
}
