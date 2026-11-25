package apiserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/internal/aivision/mock"
	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services"
)

// newIntakeAPIEnv is the API test env with the mock AI provider configured.
func newIntakeAPIEnv(c *qt.C) *trashAPIEnv {
	c.Helper()
	params, user, group := newParams()
	params.CommodityScanService = services.NewCommodityScanService(mock.New(), params.FactorySet.CommodityScanAuditRegistry,
		services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20, RateLimitPerHour: 100})
	return &trashAPIEnv{c: c, params: params, handler: apiserver.APIServer(params, &mockRestoreWorker{}), user: user, group: group}
}

// uploadIntakeFile stores a photo of the group with its blob.
func uploadIntakeFile(c *qt.C, env *trashAPIEnv, name string) *models.FileEntity {
	c.Helper()
	ctx := context.Background()
	key := "t/intake-api/" + name + ".jpg"
	file := must.Must(getRegistrySetFromParams(env.params, env.user).FileRegistry.Create(ctx, models.FileEntity{
		Title:    name,
		Type:     models.FileTypeImage,
		Category: models.FileCategoryImages,
		File:     &models.File{Path: name, OriginalPath: key, Ext: ".jpg", MIMEType: "image/jpeg", SizeBytes: 32},
	}))
	bucket := must.Must(blob.OpenBucket(ctx, env.params.UploadLocation))
	defer bucket.Close()
	c.Assert(bucket.WriteAll(ctx, key, make([]byte, 32), nil), qt.IsNil)
	return file
}

func intakeCreateBody(key string, sources ...string) string {
	return `{"data":{"type":"intake_sessions","attributes":{"client_key":"` + key + `","sources":[` + strings.Join(sources, ",") + `]}}}`
}

func intakeSource(fileIDs ...string) string {
	return `{"file_ids":["` + strings.Join(fileIDs, `","`) + `"],"one_item":false}`
}

func decodeIntake(c *qt.C, body []byte) jsonapi.IntakeSessionResponse {
	c.Helper()
	var resp jsonapi.IntakeSessionResponse
	c.Assert(json.Unmarshal(body, &resp), qt.IsNil)
	return resp
}

func TestIntakeAPI_Lifecycle(t *testing.T) {
	c := qt.New(t)
	env := newIntakeAPIEnv(c)
	receipt := uploadIntakeFile(c, env, "receipt-api")
	drill := uploadIntakeFile(c, env, "drill-api")

	rr := env.do(http.MethodGet, "/intake/limits", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.enabled"), true)
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.max_files_per_source"), float64(5))

	body := intakeCreateBody("api-batch", intakeSource(receipt.ID), intakeSource(drill.ID))
	rr = env.do(http.MethodPost, "/intake/sessions", body)
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	created := decodeIntake(c, rr.Body.Bytes())
	sessionID := created.Data.ID
	c.Check(created.Data.Attributes.State, qt.Equals, models.IntakeSessionProcessing)
	c.Assert(created.Data.Attributes.Sources, qt.HasLen, 2)
	c.Check(created.Data.Attributes.Sources[0].Files[0].URL, qt.Not(qt.Equals), "")

	rr = env.do(http.MethodPost, "/intake/sessions", body)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(decodeIntake(c, rr.Body.Bytes()).Data.ID, qt.Equals, sessionID)

	worker := services.NewIntakeWorker(env.params.FactorySet, env.params.CommodityScanService, env.params.UploadLocation)
	c.Assert(worker.Tick(context.Background()), qt.Equals, 2)

	rr = env.do(http.MethodGet, "/intake/sessions/"+sessionID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	detail := decodeIntake(c, rr.Body.Bytes())
	c.Check(detail.Data.Attributes.State, qt.Equals, models.IntakeSessionReview)
	c.Assert(detail.Data.Attributes.Candidates, qt.HasLen, 4)
	var frotherID, machineID, receiptSourceID string
	for _, cand := range detail.Data.Attributes.Candidates {
		switch cand.Fields["name"].Value {
		case "Milk Frother":
			frotherID = cand.ID
			receiptSourceID = cand.SourceID
		case "Espresso Machine":
			machineID = cand.ID
		}
	}
	c.Assert(frotherID, qt.Not(qt.Equals), "")

	review := `{"data":{"type":"intake_candidates","attributes":{"selected":false}}}`
	rr = env.do(http.MethodPatch, "/intake/sessions/"+sessionID+"/candidates/"+frotherID, review)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.selected"), false)

	rr = env.do(http.MethodPatch, "/intake/sessions/"+sessionID+"/candidates/"+frotherID,
		`{"data":{"type":"intake_candidates","attributes":{"quantity":0}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))

	rr = env.do(http.MethodPost, "/intake/sessions/"+sessionID+"/drafts", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.meta.created"), float64(3))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.meta.skipped"), float64(1))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.state"), "done")

	// A retry makes nothing twice.
	rr = env.do(http.MethodPost, "/intake/sessions/"+sessionID+"/drafts", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.meta.created"), float64(0))
	rr = env.do(http.MethodGet, "/drafts", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.meta.drafts"), float64(3))

	rr = env.do(http.MethodPatch, "/intake/sessions/"+sessionID+"/candidates/"+machineID, review)
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].code"), apiserver.IntakeCandidateFinishedCode)

	// The member changes their mind about the skipped frother.
	rr = env.do(http.MethodPatch, "/intake/sessions/"+sessionID+"/candidates/"+frotherID,
		`{"data":{"type":"intake_candidates","attributes":{"selected":true}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.outcome"), "pending")
	rr = env.do(http.MethodPost, "/intake/sessions/"+sessionID+"/drafts", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.meta.created"), float64(1))

	rr = env.do(http.MethodPost, "/intake/sessions/"+sessionID+"/sources/"+receiptSourceID+"/rescan", "")
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].code"), apiserver.IntakeSourceHasDraftsCode)

	rr = env.do(http.MethodGet, "/intake/sessions", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.meta.sessions"), float64(1))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data[0].summary.drafts_created"), float64(4))

	// A viewer reads but does not change.
	env.setRole(models.GroupRoleViewer)
	rr = env.do(http.MethodGet, "/intake/sessions/"+sessionID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodDelete, "/intake/sessions/"+sessionID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf("body=%s", rr.Body.String()))
	env.setRole(models.GroupRoleOwner)

	rr = env.do(http.MethodDelete, "/intake/sessions/"+sessionID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodGet, "/intake/sessions/"+sessionID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodGet, "/drafts", "")
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.meta.drafts"), float64(4), qt.Commentf("drafts outlive the session"))
}

func TestIntakeAPI_CreateErrors(t *testing.T) {
	c := qt.New(t)

	disabled := newTrashAPIEnv(c)
	file := uploadIntakeFile(c, disabled, "off")
	rr := disabled.do(http.MethodPost, "/intake/sessions", intakeCreateBody("off", intakeSource(file.ID)))
	c.Assert(rr.Code, qt.Equals, http.StatusServiceUnavailable, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].code"), "commodity_scan.provider_disabled")
	rr = disabled.do(http.MethodGet, "/intake/limits", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.enabled"), false)

	env := newIntakeAPIEnv(c)
	photo := uploadIntakeFile(c, env, "photo")
	tests := []struct {
		name   string
		body   string
		status int
	}{
		{name: "no data", body: `{}`, status: http.StatusUnprocessableEntity},
		{name: "wrong type", body: `{"data":{"type":"drafts","attributes":{"client_key":"k","sources":[]}}}`, status: http.StatusUnprocessableEntity},
		{name: "no sources", body: intakeCreateBody("k1"), status: http.StatusUnprocessableEntity},
		{name: "file twice", body: intakeCreateBody("k2", intakeSource(photo.ID, photo.ID)), status: http.StatusUnprocessableEntity},
		{name: "unknown file", body: intakeCreateBody("k3", intakeSource("missing")), status: http.StatusNotFound},
	}
	for _, tc := range tests {
		c.Run(tc.name, func(c *qt.C) {
			rr := env.do(http.MethodPost, "/intake/sessions", tc.body)
			c.Check(rr.Code, qt.Equals, tc.status, qt.Commentf("body=%s", rr.Body.String()))
		})
	}
}
