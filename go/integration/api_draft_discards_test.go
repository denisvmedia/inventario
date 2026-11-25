package integration_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/jackc/pgx/v5"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// TestAPIDrafts_ListFinishesStoppedDiscardsForWriters: GET /drafts finishes a
// discard that stopped more than ten minutes ago, but only when the member
// listing the drafts may discard them. A viewer's list leaves the draft as
// it is; the list of a member with the user role finishes it.
func TestAPIDrafts_ListFinishesStoppedDiscardsForWriters(t *testing.T) {
	server, fs, owner, viewer, jwtSecret, cleanup := setupTestAPIServer(t)
	defer cleanup()
	c := qt.New(t)
	ctx := context.Background()

	groupService := services.NewGroupService(fs.LocationGroupRegistry, fs.GroupMembershipRegistry, fs.GroupInviteRegistry)
	groupService.SetUserRegistry(fs.UserRegistry)
	_, err := groupService.AddMember(ctx, owner.user.TenantID, owner.group.ID, viewer.user.ID, models.GroupRoleViewer)
	c.Assert(err, qt.IsNil)
	member := apiIsolationFixture{user: mustCreateAPIUser(c, fs, owner.user.TenantID, fmt.Sprintf("member-%d@api-test.com", time.Now().UnixNano()))}
	_, err = groupService.AddMember(ctx, owner.user.TenantID, owner.group.ID, member.user.ID, models.GroupRoleUser)
	c.Assert(err, qt.IsNil)

	drafts := must.Must(fs.CommodityDraftRegistryFactory.CreateUserRegistry(owner.ctx))
	draft, _ := must.Must2(drafts.CreateOrGet(owner.ctx, models.CommodityDraft{
		ClientKey: "stopped-discard",
		Values:    models.CommodityDraftValues{"name": "Drill"},
	}))
	_, err = drafts.MarkDiscarding(owner.ctx, draft.ID)
	c.Assert(err, qt.IsNil)
	backdateDraft(c, mustTestDSN(t), draft.ID, time.Now().Add(-time.Hour))

	listDrafts := func(user apiIsolationFixture) {
		c.Helper()
		token := must.Must(generateJWTToken(user.user, jwtSecret))
		resp, err := makeAuthenticatedRequest(http.MethodGet, server.URL+"/api/v1/g/"+owner.group.Slug+"/drafts", nil, token)
		c.Assert(err, qt.IsNil)
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		c.Assert(resp.StatusCode, qt.Equals, http.StatusOK, qt.Commentf("body=%s", body))
	}

	listDrafts(viewer)
	stored, err := drafts.Get(owner.ctx, draft.ID)
	c.Assert(err, qt.IsNil, qt.Commentf("a viewer's list finishes no discard"))
	c.Check(stored.Status, qt.Equals, models.CommodityDraftDiscarding)

	listDrafts(member)
	_, err = drafts.Get(owner.ctx, draft.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound, qt.Commentf("a user's list finishes the discard"))
}

// backdateDraft sets the draft's updated_at, which is when its discard
// began, so the discard counts as stopped. The registries always write the
// current time, so the test writes the row directly.
func backdateDraft(c *qt.C, dsn, draftID string, at time.Time) {
	c.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	c.Assert(err, qt.IsNil)
	defer conn.Close(ctx)
	tag, err := conn.Exec(ctx, `UPDATE commodity_drafts SET updated_at = $1 WHERE id = $2`, at, draftID)
	c.Assert(err, qt.IsNil)
	c.Assert(tag.RowsAffected(), qt.Equals, int64(1))
}
