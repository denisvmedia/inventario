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
	must.Assert(userTemplate.SetPassword("TestPassword123"))
	user := must.Must(factorySet.UserRegistry.Create(context.Background(), userTemplate))

	var group *models.LocationGroup
	if groupCurrency != "" {
		slug := must.Must(models.GenerateGroupSlug())
		group = must.Must(factorySet.LocationGroupRegistry.Create(context.Background(), models.LocationGroup{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
			Slug:                slug,
			Name:                "Test Group",
			Status:              models.LocationGroupStatusActive,
			CreatedBy:           user.ID,
			GroupCurrency:       groupCurrency,
		}))
		must.Must(factorySet.GroupMembershipRegistry.Create(context.Background(), models.GroupMembership{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
			GroupID:             group.ID,
			MemberUserID:        user.ID,
			// Post-#1533 the group creator is the owner — every other
			// role is reachable only via invite or promotion. Tests
			// that exercise admin-but-not-owner gates set the role
			// explicitly after this helper runs.
			Role: models.GroupRoleOwner,
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

func TestGroupsAPI_CreateGroup_WithExplicitGroupCurrency(t *testing.T) {
	c := qt.New(t)

	// No pre-existing group — we're testing the create path.
	env := newGroupTestEnv(t, "")

	resp := postGroup(t, env, map[string]any{
		"name":           "Brand New",
		"icon":           "🏠",
		"group_currency": "EUR",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusCreated, qt.Commentf("body: %s", resp.Body.String()))

	var out jsonapi.LocationGroupResponse
	c.Assert(json.Unmarshal(resp.Body.Bytes(), &out), qt.IsNil)
	c.Assert(out.Data.Attributes.Name, qt.Equals, "Brand New")
	c.Assert(out.Data.Attributes.GroupCurrency, qt.Equals, models.Currency("EUR"))
}

func TestGroupsAPI_CreateGroup_DefaultsGroupCurrencyToUSD(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, "")

	resp := postGroup(t, env, map[string]any{
		"name": "No-currency group",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusCreated, qt.Commentf("body: %s", resp.Body.String()))

	var out jsonapi.LocationGroupResponse
	c.Assert(json.Unmarshal(resp.Body.Bytes(), &out), qt.IsNil)
	c.Assert(out.Data.Attributes.GroupCurrency, qt.Equals, models.Currency("USD"))
}

func TestGroupsAPI_CreateGroup_InvalidGroupCurrencyReturnsBadRequest(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, "")

	resp := postGroup(t, env, map[string]any{
		"name":           "Bad currency group",
		"group_currency": "NOPE",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusBadRequest, qt.Commentf("body: %s", resp.Body.String()))

	// No group was persisted on the failed request.
	groups := must.Must(env.factorySet.LocationGroupRegistry.List(context.Background()))
	c.Assert(groups, qt.HasLen, 0)
}

func TestGroupsAPI_UpdateGroup_RejectsGroupCurrencyChange(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, models.Currency("USD"))

	resp := patchGroup(t, env, map[string]any{
		"name":           env.group.Name,
		"group_currency": "EUR",
	})
	// #202 tracks the currency-migration tool. Until it lands, rejecting
	// loudly is better than silently dropping the change.
	c.Assert(resp.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body: %s", resp.Body.String()))

	current := must.Must(env.factorySet.LocationGroupRegistry.Get(context.Background(), env.group.ID))
	c.Assert(current.GroupCurrency, qt.Equals, models.Currency("USD"))
}

func TestGroupsAPI_UpdateGroup_SameGroupCurrencyIsNoOp(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, models.Currency("USD"))

	resp := patchGroup(t, env, map[string]any{
		"name":           "Renamed",
		"group_currency": "USD",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", resp.Body.String()))

	current := must.Must(env.factorySet.LocationGroupRegistry.Get(context.Background(), env.group.ID))
	c.Assert(current.Name, qt.Equals, "Renamed")
	c.Assert(current.GroupCurrency, qt.Equals, models.Currency("USD"))
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

// TestGroupsAPI_CreateGroup_PersistsDescription pins the create-side
// round-trip for the description field added by #1647. Empty string is
// the unset value (omitempty on the wire) — the explicit non-empty case
// here makes sure the attribute survives the apiserver → service →
// registry chain.
func TestGroupsAPI_CreateGroup_PersistsDescription(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, "")

	resp := postGroup(t, env, map[string]any{
		"name":        "Group With Description",
		"description": "Household items shared by the family",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusCreated, qt.Commentf("body: %s", resp.Body.String()))

	groups := must.Must(env.factorySet.LocationGroupRegistry.List(context.Background()))
	c.Assert(groups, qt.HasLen, 1)
	c.Assert(groups[0].Description, qt.Equals, "Household items shared by the family")
}

// TestGroupsAPI_CreateGroup_RejectsDescriptionOverCap pins the 200-char
// validation cap from jsonapi.LocationGroupAttributes — anything longer
// surfaces as 422 at bind time instead of being silently truncated.
func TestGroupsAPI_CreateGroup_RejectsDescriptionOverCap(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, "")

	tooLong := make([]byte, 201)
	for i := range tooLong {
		tooLong[i] = 'a'
	}
	resp := postGroup(t, env, map[string]any{
		"name":        "Over Cap",
		"description": string(tooLong),
	})
	c.Assert(resp.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body: %s", resp.Body.String()))
}

// TestGroupsAPI_UpdateGroup_UpdatesDescription is the patch-side
// counterpart — admin PATCH can set, change, and clear the description.
// Clearing happens with an explicit empty string on the wire (omitempty
// only drops the field when serializing; on the way in we accept "").
func TestGroupsAPI_UpdateGroup_UpdatesDescription(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, models.Currency("USD"))

	// Seed a starting description directly on the row so the test isn't
	// coupled to create-time wiring (covered by the create test above).
	env.group.Description = "Initial subtitle"
	must.Must(env.factorySet.LocationGroupRegistry.Update(context.Background(), *env.group))

	// Patch with a new value.
	resp := patchGroup(t, env, map[string]any{
		"name":        env.group.Name,
		"description": "Updated subtitle",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", resp.Body.String()))

	current := must.Must(env.factorySet.LocationGroupRegistry.Get(context.Background(), env.group.ID))
	c.Assert(current.Description, qt.Equals, "Updated subtitle")

	// Clearing the field round-trips as the empty string. The frontend
	// admin form sends "" when the textarea is emptied, so this is the
	// realistic clear path — not omitting the key from the body.
	resp = patchGroup(t, env, map[string]any{
		"name":        env.group.Name,
		"description": "",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", resp.Body.String()))

	current = must.Must(env.factorySet.LocationGroupRegistry.Get(context.Background(), env.group.ID))
	c.Assert(current.Description, qt.Equals, "")
}

// TestGroupsAPI_UpdateGroup_RejectsDescriptionOverCap mirrors the create
// guard at PATCH time. Important because the frontend never enforces the
// cap before send.
func TestGroupsAPI_UpdateGroup_RejectsDescriptionOverCap(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, models.Currency("USD"))

	tooLong := make([]byte, 201)
	for i := range tooLong {
		tooLong[i] = 'a'
	}
	resp := patchGroup(t, env, map[string]any{
		"name":        env.group.Name,
		"description": string(tooLong),
	})
	c.Assert(resp.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body: %s", resp.Body.String()))

	current := must.Must(env.factorySet.LocationGroupRegistry.Get(context.Background(), env.group.ID))
	c.Assert(current.Description, qt.Equals, "")
}

// TestGroupsAPI_GetGroup_SurfacesDescription ensures the field is part of
// the GET response payload (not just stored on the row). The frontend
// reads description off the response of /groups and /groups/{id} to
// render the sidebar subtitle.
func TestGroupsAPI_GetGroup_SurfacesDescription(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, models.Currency("USD"))
	env.group.Description = "Visible on the wire"
	must.Must(env.factorySet.LocationGroupRegistry.Update(context.Background(), *env.group))

	req := httptest.NewRequest(http.MethodGet, "/groups/"+env.group.ID, http.NoBody)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", w.Body.String()))

	var got jsonapi.LocationGroupResponse
	c.Assert(json.Unmarshal(w.Body.Bytes(), &got), qt.IsNil)
	c.Assert(got.Data, qt.IsNotNil)
	c.Assert(got.Data.Attributes, qt.IsNotNil)
	c.Assert(got.Data.Attributes.Description, qt.Equals, "Visible on the wire")
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
		"password":     "TestPassword123",
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
		"password":     "TestPassword123",
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

// Issue #1650: /groups list payloads must carry `members_count` so the
// sidebar GroupSelector renders `N member(s)` without fetching the full
// members list per group switch.
func TestGroupsAPI_ListGroups_IncludesMembersCount(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, models.Currency("USD"))

	// Add a second member so the count is non-trivial (≠ 1 from the
	// owner alone) and we can distinguish "the field is wired" from
	// "the field happens to coincide with the default seed".
	secondUser := must.Must(env.factorySet.UserRegistry.Create(context.Background(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: env.user.TenantID},
		Email:               "second@test.local",
		Name:                "Second User",
		IsActive:            true,
	}))
	must.Must(env.factorySet.GroupMembershipRegistry.Create(context.Background(), models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: env.user.TenantID},
		GroupID:             env.group.ID,
		MemberUserID:        secondUser.ID,
		Role:                models.GroupRoleUser,
	}))

	req := httptest.NewRequest(http.MethodGet, "/groups", nil)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", w.Body.String()))

	var out jsonapi.LocationGroupsResponse
	c.Assert(json.Unmarshal(w.Body.Bytes(), &out), qt.IsNil)
	c.Assert(out.Data, qt.HasLen, 1)
	c.Assert(out.Data[0].Attributes.MembersCount, qt.Equals, 2)
}

func TestGroupsAPI_GetGroup_IncludesMembersCount(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, models.Currency("USD"))

	req := httptest.NewRequest(http.MethodGet, "/groups/"+env.group.ID, nil)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", w.Body.String()))

	var out jsonapi.LocationGroupResponse
	c.Assert(json.Unmarshal(w.Body.Bytes(), &out), qt.IsNil)
	// Only the seeded owner is a member of this fresh group.
	c.Assert(out.Data.Attributes.MembersCount, qt.Equals, 1)
}

// Issue #1653: /groups list payloads carry `current_user_role` so the Profile
// page Groups tab can render the caller's role per group without a per-tile
// members lookup.
func TestGroupsAPI_ListGroups_IncludesCurrentUserRole(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, models.Currency("USD"))

	req := httptest.NewRequest(http.MethodGet, "/groups", nil)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", w.Body.String()))

	var out jsonapi.LocationGroupsResponse
	c.Assert(json.Unmarshal(w.Body.Bytes(), &out), qt.IsNil)
	c.Assert(out.Data, qt.HasLen, 1)
	role := out.Data[0].Attributes.CurrentUserRole
	c.Assert(role, qt.IsNotNil)
	c.Assert(*role, qt.Equals, models.GroupRoleOwner)
}

func TestGroupsAPI_GetGroup_IncludesCurrentUserRole(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, models.Currency("USD"))

	req := httptest.NewRequest(http.MethodGet, "/groups/"+env.group.ID, nil)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", w.Body.String()))

	var out jsonapi.LocationGroupResponse
	c.Assert(json.Unmarshal(w.Body.Bytes(), &out), qt.IsNil)
	role := out.Data.Attributes.CurrentUserRole
	c.Assert(role, qt.IsNotNil)
	c.Assert(*role, qt.Equals, models.GroupRoleOwner)
}

func TestGroupsAPI_ListGroups_CurrentUserRoleMatchesPerGroupMembership(t *testing.T) {
	c := qt.New(t)

	env := newGroupTestEnv(t, models.Currency("USD"))

	// Add a second group where the caller is just a `user`, not an owner.
	// listGroups returns every group the user belongs to — verify the role
	// is sourced per-group, not from a shared bucket.
	slug := must.Must(models.GenerateGroupSlug())
	secondGroup := must.Must(env.factorySet.LocationGroupRegistry.Create(context.Background(), models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: env.user.TenantID},
		Slug:                slug,
		Name:                "Second Group",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           env.user.ID,
		GroupCurrency:       models.Currency("USD"),
	}))
	must.Must(env.factorySet.GroupMembershipRegistry.Create(context.Background(), models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: env.user.TenantID},
		GroupID:             secondGroup.ID,
		MemberUserID:        env.user.ID,
		Role:                models.GroupRoleUser,
	}))

	req := httptest.NewRequest(http.MethodGet, "/groups", nil)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", w.Body.String()))

	var out jsonapi.LocationGroupsResponse
	c.Assert(json.Unmarshal(w.Body.Bytes(), &out), qt.IsNil)
	c.Assert(out.Data, qt.HasLen, 2)
	byID := make(map[string]*models.GroupRole, 2)
	for _, item := range out.Data {
		byID[item.ID] = item.Attributes.CurrentUserRole
	}
	c.Assert(byID[env.group.ID], qt.IsNotNil)
	c.Assert(*byID[env.group.ID], qt.Equals, models.GroupRoleOwner)
	c.Assert(byID[secondGroup.ID], qt.IsNotNil)
	c.Assert(*byID[secondGroup.ID], qt.Equals, models.GroupRoleUser)
}
