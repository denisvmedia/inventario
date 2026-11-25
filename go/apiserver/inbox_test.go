package apiserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

func decodeInbox(c *qt.C, body []byte) jsonapi.InboxResponse {
	c.Helper()
	var resp jsonapi.InboxResponse
	c.Assert(json.Unmarshal(body, &resp), qt.IsNil)
	return resp
}

func TestInboxAPI_ListAndCount(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	regs := getRegistrySetFromParams(env.params, env.user)
	ctx := context.Background()
	areaID := must.Must(regs.AreaRegistry.List(ctx))[0].ID

	rr := env.do(http.MethodGet, "/inbox/count", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.total"), float64(0))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.reasons.draft"), float64(0))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.reasons.item_draft"), float64(0))

	rr = env.do(http.MethodPost, "/drafts", draftCreateBody("key-1", "Drill"))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	draftID := decodeDraft(c, rr.Body.Bytes()).Data.ID
	lamp := must.Must(regs.CommodityRegistry.Create(ctx, models.Commodity{
		Name:   "Lamp",
		Type:   models.CommodityTypeOther,
		Status: models.CommodityStatusInUse,
		Count:  1,
		AreaID: new(areaID),
		Draft:  true,
	}))

	rr = env.do(http.MethodGet, "/inbox", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Header().Get("X-Total"), qt.Equals, "2")
	inbox := decodeInbox(c, rr.Body.Bytes())
	c.Assert(inbox.Data, qt.HasLen, 2)
	c.Check(inbox.Data[0].Kind, qt.Equals, "draft")
	c.Check(inbox.Data[0].ID, qt.Equals, draftID)
	c.Check(inbox.Data[0].Title, qt.Equals, "Drill")
	c.Check(inbox.Data[0].Reasons, qt.DeepEquals, []string{"draft"})
	c.Check(inbox.Data[0].UpdatedAt, qt.IsNotNil)
	c.Check(inbox.Data[1].Kind, qt.Equals, "commodity")
	c.Check(inbox.Data[1].ID, qt.Equals, lamp.ID)
	c.Check(inbox.Data[1].Reasons, qt.DeepEquals, []string{"item_draft"})
	c.Check(inbox.Data[1].UpdatedAt, qt.IsNil)
	c.Check(inbox.Meta, qt.DeepEquals, jsonapi.InboxMeta{
		Entries: 2,
		Counts:  jsonapi.InboxCounts{Total: 2, Reasons: map[string]int{"draft": 1, "item_draft": 1}},
	})

	rr = env.do(http.MethodGet, "/inbox?reason=item_draft", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	inbox = decodeInbox(c, rr.Body.Bytes())
	c.Assert(inbox.Data, qt.HasLen, 1)
	c.Check(inbox.Data[0].ID, qt.Equals, lamp.ID)
	c.Check(inbox.Meta.Entries, qt.Equals, 1)
	c.Check(inbox.Meta.Counts.Total, qt.Equals, 2)

	rr = env.do(http.MethodGet, "/inbox?per_page=1&page=2", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	inbox = decodeInbox(c, rr.Body.Bytes())
	c.Assert(inbox.Data, qt.HasLen, 1)
	c.Check(inbox.Data[0].ID, qt.Equals, lamp.ID)

	rr = env.do(http.MethodGet, "/inbox/count", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.total"), float64(2))

	// Clearing the reasons empties the inbox: the draft is discarded and the
	// item reviewed.
	rr = env.do(http.MethodDelete, "/drafts/"+draftID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent, qt.Commentf("body=%s", rr.Body.String()))
	lamp.Draft = false
	must.Must(regs.CommodityRegistry.Update(ctx, *lamp))
	rr = env.do(http.MethodGet, "/inbox/count", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.total"), float64(0))
}

func TestInboxAPI_RefusesAnUnknownReason(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)

	rr := env.do(http.MethodGet, "/inbox?reason=no_location", "")
	c.Assert(rr.Code, qt.Equals, http.StatusBadRequest, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].code"), "query.invalid_parameter")
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].meta.parameter"), "reason")
}

func TestInboxAPI_ViewerCanRead(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	rr := env.do(http.MethodPost, "/drafts", draftCreateBody("key-1", "Drill"))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))

	env.setRole(models.GroupRoleViewer)
	rr = env.do(http.MethodGet, "/inbox", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(decodeInbox(c, rr.Body.Bytes()).Data, qt.HasLen, 1)
	rr = env.do(http.MethodGet, "/inbox/count", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.total"), float64(1))
}

func TestInboxAPI_CountsOnlyTheGroup(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	other := *env
	other.group = createTestGroupForUser(env.params.FactorySet, env.user.TenantID, env.user.ID)

	rr := other.do(http.MethodPost, "/drafts", draftCreateBody("key-1", "Other group's drill"))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodPost, "/drafts", draftCreateBody("key-1", "Drill"))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))

	rr = env.do(http.MethodGet, "/inbox/count", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.total"), float64(1))
	rr = env.do(http.MethodGet, "/inbox", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	inbox := decodeInbox(c, rr.Body.Bytes())
	c.Assert(inbox.Data, qt.HasLen, 1)
	c.Check(inbox.Data[0].Title, qt.Equals, "Drill")

	rr = other.do(http.MethodGet, "/inbox/count", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.total"), float64(1))
}

func TestInboxAPI_ListsOnlyTheGroupsDraftItems(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	other := *env
	other.group = createTestGroupForUser(env.params.FactorySet, env.user.TenantID, env.user.ID)
	ctx := context.Background()
	regs := getRegistrySetFromParams(env.params, env.user)
	otherCtx := appctx.WithGroup(createTestUserContext(env.user.ID, env.user.TenantID), other.group)
	otherRegs := must.Must(env.params.FactorySet.CreateUserRegistrySet(otherCtx))

	draftItem := func(set *registry.Set, ctx context.Context, name string) *models.Commodity {
		return must.Must(set.CommodityRegistry.Create(ctx, models.Commodity{
			Name: name, Type: models.CommodityTypeOther, Status: models.CommodityStatusInUse, Count: 1, Draft: true,
		}))
	}
	lamp := draftItem(regs, ctx, "Lamp")
	shelf := draftItem(otherRegs, otherCtx, "Other group's shelf")

	for _, tc := range []struct {
		env  *trashAPIEnv
		want *models.Commodity
	}{{env, lamp}, {&other, shelf}} {
		rr := tc.env.do(http.MethodGet, "/inbox?reason=item_draft", "")
		c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
		inbox := decodeInbox(c, rr.Body.Bytes())
		c.Assert(inbox.Data, qt.HasLen, 1)
		c.Check(inbox.Data[0].ID, qt.Equals, tc.want.ID)
		c.Check(inbox.Meta.Counts.Reasons["item_draft"], qt.Equals, 1)
		rr = tc.env.do(http.MethodGet, "/inbox/count", "")
		c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
		c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.reasons.item_draft"), float64(1))
	}
}

// A discard that stopped half way keeps the draft's files and client key.
// Reading the inbox finishes it once it is ten minutes old, as GET /drafts
// does, when the reader may discard drafts.
func TestInboxAPI_FinishesStoppedDiscards(t *testing.T) {
	for _, path := range []string{"/inbox", "/inbox/count"} {
		t.Run(path, func(t *testing.T) {
			c := qt.New(t)
			env := newTrashAPIEnv(c)
			env.params.CommodityDraftServiceOptions = []services.CommodityDraftServiceOption{
				services.WithCommodityDraftClock(func() time.Time { return time.Now().Add(11 * time.Minute) }),
			}
			env.handler = apiserver.APIServer(env.params, &mockRestoreWorker{})
			regs := getRegistrySetFromParams(env.params, env.user)
			ctx := context.Background()

			rr := env.do(http.MethodPost, "/drafts", draftCreateBody("key-1", "Drill"))
			c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
			id := decodeDraft(c, rr.Body.Bytes()).Data.ID
			must.Must(regs.CommodityDraftRegistry.MarkDiscarding(ctx, id))

			// A viewer cannot discard drafts, so reading leaves it.
			env.setRole(models.GroupRoleViewer)
			rr = env.do(http.MethodGet, path, "")
			c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
			stopped := must.Must(regs.CommodityDraftRegistry.Get(ctx, id))
			c.Check(stopped.Status, qt.Equals, models.CommodityDraftDiscarding)

			env.setRole(models.GroupRoleUser)
			rr = env.do(http.MethodGet, path, "")
			c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
			_, err := regs.CommodityDraftRegistry.Get(ctx, id)
			c.Check(err, qt.ErrorIs, registry.ErrNotFound)
			// The client key is free again.
			rr = env.do(http.MethodPost, "/drafts", draftCreateBody("key-1", "Drill again"))
			c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
		})
	}
}

// A discard younger than ten minutes may still be running, so reading the
// inbox leaves it alone.
func TestInboxAPI_LeavesARecentDiscard(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	regs := getRegistrySetFromParams(env.params, env.user)
	ctx := context.Background()
	rr := env.do(http.MethodPost, "/drafts", draftCreateBody("key-1", "Drill"))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	id := decodeDraft(c, rr.Body.Bytes()).Data.ID
	must.Must(regs.CommodityDraftRegistry.MarkDiscarding(ctx, id))

	rr = env.do(http.MethodGet, "/inbox/count", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.total"), float64(0))
	running := must.Must(regs.CommodityDraftRegistry.Get(ctx, id))
	c.Check(running.Status, qt.Equals, models.CommodityDraftDiscarding)
}
