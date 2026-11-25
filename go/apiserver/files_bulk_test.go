package apiserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/models"
)

// bulkFile creates a live file in the env's group.
func (e *trashAPIEnv) bulkFile(title string, tags ...string) *models.FileEntity {
	e.c.Helper()
	ctx := createTestUserContextWithGroup(e.user.ID, e.user.TenantID, e.group.ID)
	set := must.Must(e.params.FactorySet.CreateUserRegistrySet(ctx))
	return must.Must(set.FileRegistry.Create(ctx, models.FileEntity{
		Title:    title,
		Type:     models.FileTypeDocument,
		Category: models.FileCategoryDocuments,
		Tags:     tags,
		File: &models.File{
			Path:         title,
			OriginalPath: "blobs/" + title + ".pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))
}

func (e *trashAPIEnv) fileTags(id string) []string {
	e.c.Helper()
	return []string(must.Must(getRegistrySetFromParams(e.params, e.user).FileRegistry.Get(context.Background(), id)).Tags)
}

func fileBulkBody(attributes map[string]any) string {
	return string(must.Must(json.Marshal(map[string]any{
		"data": map[string]any{"type": "files", "attributes": attributes},
	})))
}

func TestFileBulkUpdate_Tags(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	manual := env.bulkFile("manual", "manual", "old")
	photo := env.bulkFile("photo")
	receipt := env.bulkFile("receipt", "receipt")

	rr := env.do(http.MethodPost, "/files/bulk-update", fileBulkBody(map[string]any{
		"ids":   []string{manual.ID, photo.ID, receipt.ID, "missing"},
		"patch": map[string]any{"tags": map[string]any{"add": []string{"Receipt"}, "remove": []string{"old"}}},
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	body := rr.Body.String()
	c.Check(body, checkers.JSONPathEquals("$.data.type"), "files.bulk_result")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.succeeded"), []any{manual.ID, photo.ID})
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.skipped[0].id"), receipt.ID)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.skipped[0].name"), "receipt")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.failed[0].id"), "missing")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.failed[0].code"), "not_found")

	// Tags the patch does not name stay on each file.
	c.Assert(env.fileTags(manual.ID), qt.DeepEquals, []string{"manual", "receipt"})
	c.Assert(env.fileTags(photo.ID), qt.DeepEquals, []string{"receipt"})
}

func TestFileBulkUpdate_Refused(t *testing.T) {
	ids101 := make([]string, 101)
	for i := range ids101 {
		ids101[i] = "id"
	}
	addTag := map[string]any{"tags": map[string]any{"add": []string{"receipt"}}}
	tests := []struct {
		name  string
		attrs map[string]any
	}{
		{"no ids", map[string]any{"patch": addTag}},
		{"more than 100 ids", map[string]any{"ids": ids101, "patch": addTag}},
		{"no patch", map[string]any{"ids": []string{"a"}}},
		{"no tags", map[string]any{"ids": []string{"a"}, "patch": map[string]any{"tags": map[string]any{}}}},
		{"a tag both added and removed", map[string]any{"ids": []string{"a"}, "patch": map[string]any{
			"tags": map[string]any{"add": []string{"Receipt"}, "remove": []string{"receipt"}},
		}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			env := newTrashAPIEnv(c)
			rr := env.do(http.MethodPost, "/files/bulk-update", fileBulkBody(tt.attrs))
			c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
		})
	}
}

func TestFileBulkUpdate_ViewerIsRefused(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	file := env.bulkFile("viewer")
	env.setRole(models.GroupRoleViewer)

	rr := env.do(http.MethodPost, "/files/bulk-update", fileBulkBody(map[string]any{
		"ids": []string{file.ID}, "patch": map[string]any{"tags": map[string]any{"add": []string{"receipt"}}},
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf("body=%s", rr.Body.String()))
	c.Assert(env.fileTags(file.ID), qt.HasLen, 0)
}
