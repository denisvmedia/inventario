package apiserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services"
)

// Every restore and delete for good in Trash writes one audit row with the
// member, the group, the operator when the member is impersonated, and the
// items and files it restored or deleted (#2893).

// trashAuditExtra is the breadcrumb a Trash audit row carries.
type trashAuditExtra struct {
	Items     int      `json:"items"`
	ItemIDs   []string `json:"item_ids"`
	Files     int      `json:"files"`
	FileIDs   []string `json:"file_ids"`
	Failed    int      `json:"failed"`
	FailedIDs []string `json:"failed_ids"`
}

// trashAuditRows returns the audit rows of action with their breadcrumbs.
func (e *trashAPIEnv) trashAuditRows(action string) ([]*models.AuditLog, []trashAuditExtra) {
	e.c.Helper()
	all := must.Must(e.params.FactorySet.AuditLogRegistry.List(context.Background()))
	var rows []*models.AuditLog
	var extras []trashAuditExtra
	for _, row := range all {
		if row.Action != action {
			continue
		}
		var extra trashAuditExtra
		e.c.Assert(json.Unmarshal([]byte(row.UserAgent), &extra), qt.IsNil, qt.Commentf("user_agent=%s", row.UserAgent))
		rows = append(rows, row)
		extras = append(extras, extra)
	}
	return rows, extras
}

// linkedFileIDs are the files linked to the item, sorted.
func (e *trashAPIEnv) linkedFileIDs(itemID string) []string {
	e.c.Helper()
	ctx := createTestUserContextWithGroup(e.user.ID, e.user.TenantID, e.group.ID)
	set := must.Must(e.params.FactorySet.CreateUserRegistrySet(ctx))
	files := must.Must(set.FileRegistry.ListByLinkedEntity(ctx, models.FileLinkCommodity, itemID))
	ids := make([]string, 0, len(files))
	for _, f := range files {
		ids = append(ids, f.ID)
	}
	slices.Sort(ids)
	e.c.Assert(ids, qt.Not(qt.HasLen), 0, qt.Commentf("the fixture item needs files"))
	return ids
}

func sorted(ids []string) []string {
	out := slices.Clone(ids)
	slices.Sort(out)
	return out
}

func (e *trashAPIEnv) assertTrashRow(row *models.AuditLog, success bool) {
	e.c.Helper()
	e.c.Assert(row.UserID, qt.IsNotNil)
	e.c.Assert(*row.UserID, qt.Equals, e.user.ID)
	e.c.Assert(*row.TenantID, qt.Equals, e.group.TenantID)
	e.c.Assert(*row.EntityType, qt.Equals, "location_group")
	e.c.Assert(*row.EntityID, qt.Equals, e.group.ID)
	e.c.Assert(row.Success, qt.Equals, success)
}

func TestTrashAPI_PurgeWritesAnAuditRow(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	ids := env.commodityIDs()
	files := env.linkedFileIDs(ids[0])

	c.Assert(env.do(http.MethodDelete, "/commodities/"+ids[0], "").Code, qt.Equals, http.StatusNoContent)
	rr := env.do(http.MethodDelete, "/trash/commodities/"+ids[0], "")
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent, qt.Commentf("body=%s", rr.Body.String()))

	rows, extras := env.trashAuditRows(services.AuditActionTrashPurge)
	c.Assert(rows, qt.HasLen, 1)
	env.assertTrashRow(rows[0], true)
	c.Assert(rows[0].ImpersonatedBy, qt.IsNil)
	c.Assert(extras[0].Items, qt.Equals, 1)
	c.Assert(extras[0].ItemIDs, qt.DeepEquals, []string{ids[0]})
	c.Assert(extras[0].Files, qt.Equals, len(files))
	c.Assert(sorted(extras[0].FileIDs), qt.DeepEquals, files)
}

func TestTrashAPI_PurgeUnderImpersonationRecordsTheOperator(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	operator, operatorToken := WithBackofficeAdmin(t, env.params)
	ids := env.commodityIDs()
	c.Assert(env.do(http.MethodDelete, "/commodities/"+ids[0], "").Code, qt.Equals, http.StatusNoContent)

	start := doImpersonateStart(c, env.handler, operatorToken, env.user.ID, nil)
	c.Assert(start.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", start.Body.String()))
	var session apiserver.LoginResponse
	c.Assert(json.Unmarshal(start.Body.Bytes(), &session), qt.IsNil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/g/"+env.group.Slug+"/trash/commodities/"+ids[0], nil)
	req.Header.Set("Authorization", "Bearer "+session.AccessToken)
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent, qt.Commentf("body=%s", rr.Body.String()))

	rows, extras := env.trashAuditRows(services.AuditActionTrashPurge)
	c.Assert(rows, qt.HasLen, 1)
	env.assertTrashRow(rows[0], true)
	c.Assert(rows[0].ImpersonatedBy, qt.IsNotNil)
	c.Assert(*rows[0].ImpersonatedBy, qt.Equals, operator.ID)
	c.Assert(extras[0].ItemIDs, qt.DeepEquals, []string{ids[0]})
}

func TestTrashAPI_RestoreWritesAnAuditRow(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	ids := env.commodityIDs()
	files := env.linkedFileIDs(ids[0])

	c.Assert(env.do(http.MethodDelete, "/commodities/"+ids[0], "").Code, qt.Equals, http.StatusNoContent)
	rr := env.do(http.MethodPost, "/trash/commodities/"+ids[0]+"/restore", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))

	rows, extras := env.trashAuditRows(services.AuditActionTrashRestore)
	c.Assert(rows, qt.HasLen, 1)
	env.assertTrashRow(rows[0], true)
	c.Assert(extras[0].ItemIDs, qt.DeepEquals, []string{ids[0]})
	c.Assert(sorted(extras[0].FileIDs), qt.DeepEquals, files)
}

// A bulk request writes one row: what went through, and the IDs that did
// not.
func TestTrashAPI_BulkPurgeWritesOneAuditRow(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	ids := env.commodityIDs()

	c.Assert(env.do(http.MethodDelete, "/commodities/"+ids[0], "").Code, qt.Equals, http.StatusNoContent)
	c.Assert(env.do(http.MethodDelete, "/commodities/"+ids[1], "").Code, qt.Equals, http.StatusNoContent)
	rr := env.do(http.MethodPost, "/trash/commodities/purge",
		`{"data":{"type":"commodities","attributes":{"ids":["`+ids[0]+`","not-in-trash","`+ids[1]+`"]}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))

	rows, extras := env.trashAuditRows(services.AuditActionTrashPurge)
	c.Assert(rows, qt.HasLen, 1)
	env.assertTrashRow(rows[0], false)
	c.Assert(*rows[0].ErrorMessage, qt.Equals, "1 of 3 entries failed")
	c.Assert(extras[0].ItemIDs, qt.DeepEquals, []string{ids[0], ids[1]})
	c.Assert(extras[0].Failed, qt.Equals, 1)
	c.Assert(extras[0].FailedIDs, qt.DeepEquals, []string{"not-in-trash"})
}

func TestTrashAPI_EmptyTrashWritesAnAuditRow(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	ids := env.commodityIDs()
	files := env.linkedFileIDs(ids[0])

	c.Assert(env.do(http.MethodDelete, "/commodities/"+ids[0], "").Code, qt.Equals, http.StatusNoContent)
	rr := env.do(http.MethodDelete, "/trash", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	// The response still counts Trash entries: the files went with the item.
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.commodities"), float64(1))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.files"), float64(0))

	rows, extras := env.trashAuditRows(services.AuditActionTrashEmpty)
	c.Assert(rows, qt.HasLen, 1)
	env.assertTrashRow(rows[0], true)
	c.Assert(extras[0].ItemIDs, qt.DeepEquals, []string{ids[0]})
	c.Assert(sorted(extras[0].FileIDs), qt.DeepEquals, files)
}
