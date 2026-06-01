package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"
	"github.com/yalp/jsonpath"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/checkers"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// Currency-migration apiserver integration tests (issue #202 / #1551).
//
// All cases run against the in-memory registry with the feature flag
// enabled unless the test specifically exercises the flag-off path.
// Tests that need an actual partial unique index (concurrent-start race)
// or a daily-cap clock advance live alongside the postgres registry
// integration tests; the in-memory backend is sufficient for the rest
// of the DoD.

// newCurrencyMigrationParams returns Params with FeatureCurrencyMigration
// enabled. For "feature off" tests, callers should use newParams directly.
func newCurrencyMigrationParams() (apiserver.Params, *models.User, *models.LocationGroup) {
	params, testUser, testGroup := newParams()
	params.FeatureCurrencyMigration = true
	return params, testUser, testGroup
}

func doJSONAPIRequest(t *testing.T, handler http.Handler, method, path, userID string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}
	req, err := http.NewRequest(method, path, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.api+json")
	if userID != "" {
		addTestUserAuthHeader(req, userID)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// assertErrorCode asserts the JSON:API errors[0].code matches the given
// expected code. The FE branches on these stable codes, so locking them
// in here prevents accidental contract regressions.
func assertErrorCode(t *testing.T, c *qt.C, body []byte, expected string) {
	t.Helper()
	c.Assert(body, checkers.JSONPathEquals("$.errors[0].code"), expected)
}

// assertErrorMeta asserts errors[0].meta[key] == expected (any type).
// Used to verify retry_after_seconds (429) / migration_id+status (423)
// payloads stay stable.
func assertErrorMeta(t *testing.T, c *qt.C, body []byte, key string, predicate func(any) bool, hint string) {
	t.Helper()
	var parsed any
	c.Assert(json.Unmarshal(body, &parsed), qt.IsNil, qt.Commentf("assertErrorMeta: parse body=%s", string(body)))
	v, err := jsonpath.Read(parsed, "$.errors[0].meta."+key)
	c.Assert(err, qt.IsNil, qt.Commentf("assertErrorMeta(%s) lookup; body=%s", key, string(body)))
	c.Assert(predicate(v), qt.IsTrue, qt.Commentf("assertErrorMeta(%s): %s; got %v (%T) body=%s", key, hint, v, v, string(body)))
}

// previewBody is a minimal JSON:API request body builder to keep the
// tests readable.
func previewBody(from, to string, rate string) map[string]any {
	return map[string]any{
		"data": map[string]any{
			"type": "currency-migrations",
			"attributes": map[string]any{
				"from_currency": from,
				"to_currency":   to,
				"exchange_rate": rate,
			},
		},
	}
}

func startBody(from, to, rate, token string) map[string]any {
	return map[string]any{
		"data": map[string]any{
			"type": "currency-migrations",
			"attributes": map[string]any{
				"from_currency": from,
				"to_currency":   to,
				"exchange_rate": rate,
				"preview_token": token,
			},
		},
	}
}

func TestCurrencyMigrations_FeatureFlagOff_NotMounted(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams() // flag default false
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Every endpoint under /currency-migrations is short-circuited by
	// the featureGate middleware. The response must be a 404 with the
	// `currency_migration.feature_disabled` JSON:API code so the FE can
	// surface "feature disabled in this deployment" copy instead of
	// silently swallowing the error (#1616).
	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"preview", http.MethodPost, "/currency-migrations/preview", previewBody("USD", "EUR", "0.9")},
		{"start", http.MethodPost, "/currency-migrations", startBody("USD", "EUR", "0.9", "tok")},
		{"list", http.MethodGet, "/currency-migrations", nil},
		{"detail", http.MethodGet, "/currency-migrations/some-id", nil},
	}
	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			rr := doJSONAPIRequest(t, handler, tc.method, "/api/v1/g/"+testGroup.Slug+tc.path, testUser.ID, tc.body)
			c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
			assertErrorCode(t, c, rr.Body.Bytes(), "currency_migration.feature_disabled")
		})
	}
}

func TestCurrencyMigrations_Preview_Happy(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newCurrencyMigrationParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations/preview", testUser.ID, previewBody("USD", "EUR", "0.9"))

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()
	c.Assert(body, checkers.JSONPathEquals("$.data.type"), "currency-migration-previews")
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.from_currency"), "USD")
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.to_currency"), "EUR")
	// The token must be a non-empty string. Tighter assertions on the
	// signature / payload layout live in the registry-level tests for
	// IssuePreviewToken / VerifyPreviewToken (PR 1).
	c.Assert(body, checkers.JSONPathMatches("$.data.attributes.preview_token", qt.Not(qt.Equals)), "")
	c.Assert(body, checkers.JSONPathMatches("$.data.attributes.state_hash", qt.Not(qt.Equals)), "")
}

func TestCurrencyMigrations_Preview_SameCurrencyRejected422(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newCurrencyMigrationParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations/preview", testUser.ID, previewBody("USD", "USD", "1"))

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	// from==to is caught by the from-mismatch guard first when the
	// group currency is USD; either code is wire-stable.
	body := rr.Body.Bytes()
	c.Assert(body, checkers.JSONPathMatches("$.errors[0].code", qt.Not(qt.Equals)), "")
}

func TestCurrencyMigrations_Preview_FromMismatchRejected422(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newCurrencyMigrationParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Group's GroupCurrency is USD; submitting from=GBP must 422 with a
	// stable code so the FE can render "from currency mismatch".
	rr := doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations/preview", testUser.ID, previewBody("GBP", "EUR", "0.9"))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), "currency_migration.from_mismatch")
}

func TestCurrencyMigrations_Preview_ZeroRateRejected422(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newCurrencyMigrationParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations/preview", testUser.ID, previewBody("USD", "EUR", "0"))

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), "currency_migration.rate_invalid")
}

// Garbage to_currency must be rejected at the preview boundary with a
// stable JSON:API code so the FE can render a targeted error instead of
// the generic 422 the registry-layer ValidateWithContext used to return
// (issue #1624).
func TestCurrencyMigrations_Preview_InvalidToCurrencyRejected422(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newCurrencyMigrationParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations/preview", testUser.ID, previewBody("USD", "FOO", "0.9"))

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), "currency_migration.invalid_to_currency")
}

// Same gap on the start handler — without the ISO check, garbage
// to_currency only failed inside CurrencyMigrationRegistry.Create with a
// generic wrapped 422.
func TestCurrencyMigrations_Start_InvalidToCurrencyRejected422(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newCurrencyMigrationParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Token value is irrelevant — the IsValid guard short-circuits before
	// VerifyPreviewToken is even called.
	rr := doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations", testUser.ID, startBody("USD", "FOO", "0.9", "any-token"))

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), "currency_migration.invalid_to_currency")
}

func TestCurrencyMigrations_Start_Happy_AndCreatesPendingRow(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newCurrencyMigrationParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Preview to obtain a token + state hash bound to the group's live state.
	rr := doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations/preview", testUser.ID, previewBody("USD", "EUR", "0.9"))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	previewToken := jsonPathString(t, rr.Body.Bytes(), "$.data.attributes.preview_token")

	// Commit.
	rr = doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations", testUser.ID, startBody("USD", "EUR", "0.9", previewToken))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.type"), "currency-migrations")
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.status"), "pending")

	// The pending row must show up in the list endpoint.
	rr = doJSONAPIRequest(t, handler, http.MethodGet, "/api/v1/g/"+testGroup.Slug+"/currency-migrations", testUser.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathMatches("$.data", qt.HasLen), 1)
}

func TestCurrencyMigrations_Start_TokenInvalid_Returns422(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newCurrencyMigrationParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations", testUser.ID, startBody("USD", "EUR", "0.9", "definitely-not-a-token"))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), "currency_migration.token_invalid")
}

func TestCurrencyMigrations_Start_TokenBindingsMismatched_Returns409(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newCurrencyMigrationParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Preview at one rate, commit at a different rate.
	rr := doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations/preview", testUser.ID, previewBody("USD", "EUR", "0.9"))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	previewToken := jsonPathString(t, rr.Body.Bytes(), "$.data.attributes.preview_token")

	rr = doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations", testUser.ID, startBody("USD", "EUR", "0.95", previewToken))
	c.Assert(rr.Code, qt.Equals, http.StatusConflict)
	assertErrorCode(t, c, rr.Body.Bytes(), "currency_migration.state_changed")
}

func TestCurrencyMigrations_Start_StateDriftRejectedOn409(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newCurrencyMigrationParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations/preview", testUser.ID, previewBody("USD", "EUR", "0.9"))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	previewToken := jsonPathString(t, rr.Body.Bytes(), "$.data.attributes.preview_token")

	// Mutate the group's state — adding a commodity changes the
	// (count, sum_current_price) hash and must invalidate the token.
	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
	rs := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
	areas := must.Must(rs.AreaRegistry.List(ctx))
	c.Assert(len(areas) > 0, qt.IsTrue)
	must.Must(rs.CommodityRegistry.Create(ctx, models.Commodity{
		Name:                  "Drift test",
		Type:                  models.CommodityTypeWhiteGoods,
		Status:                models.CommodityStatusInUse,
		AreaID:                new(areas[0].ID),
		Count:                 1,
		OriginalPrice:         decimal.RequireFromString("100"),
		OriginalPriceCurrency: models.Currency("USD"),
		CurrentPrice:          decimal.RequireFromString("100"),
	}))

	rr = doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations", testUser.ID, startBody("USD", "EUR", "0.9", previewToken))
	c.Assert(rr.Code, qt.Equals, http.StatusConflict)
	assertErrorCode(t, c, rr.Body.Bytes(), "currency_migration.state_changed")
}

func TestCurrencyMigrations_Start_InFlightMigrationRejected409(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newCurrencyMigrationParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Plant a pending migration row so the second start is the loser.
	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
	cmReg := must.Must(params.FactorySet.CurrencyMigrationRegistryFactory.CreateUserRegistry(ctx))
	must.Must(cmReg.Create(ctx, models.CurrencyMigration{
		FromCurrency: models.Currency("USD"),
		ToCurrency:   models.Currency("EUR"),
		ExchangeRate: decimal.RequireFromString("0.9"),
		Status:       models.CurrencyMigrationStatusPending,
	}))

	// Preview still succeeds (read-only) but start should 409.
	rr := doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations/preview", testUser.ID, previewBody("USD", "EUR", "0.9"))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	previewToken := jsonPathString(t, rr.Body.Bytes(), "$.data.attributes.preview_token")

	rr = doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations", testUser.ID, startBody("USD", "EUR", "0.9", previewToken))
	c.Assert(rr.Code, qt.Equals, http.StatusConflict)
	body := rr.Body.Bytes()
	assertErrorCode(t, c, body, "currency_migration.migration_in_progress")
	// Meta should carry the in-flight migration's id and status so the
	// FE can deep-link to the lock UX without a second roundtrip.
	assertErrorMeta(t, c, body, "migration_id", func(v any) bool { s, _ := v.(string); return s != "" }, "expected non-empty migration_id")
	assertErrorMeta(t, c, body, "status", func(v any) bool { return v == "pending" || v == "running" }, "expected pending|running")
}

func TestCurrencyMigrations_Start_RestoreInFlightRejected409(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newCurrencyMigrationParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Plant a pending restore_operation in this group.
	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
	rs := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
	exports, err := rs.ExportRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	exportID := ""
	if len(exports) > 0 {
		exportID = exports[0].ID
	} else {
		// Create a minimal completed export to satisfy the FK on RestoreOperation.ExportID.
		exp := must.Must(rs.ExportRegistry.Create(ctx, models.Export{
			Type:        models.ExportTypeFullDatabase,
			Description: "test export",
			Status:      models.ExportStatusCompleted,
		}))
		exportID = exp.ID
	}
	must.Must(rs.RestoreOperationRegistry.Create(ctx, models.RestoreOperation{
		ExportID:    exportID,
		Description: "manual planted restore",
		Status:      models.RestoreStatusPending,
	}))

	rr := doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations/preview", testUser.ID, previewBody("USD", "EUR", "0.9"))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	previewToken := jsonPathString(t, rr.Body.Bytes(), "$.data.attributes.preview_token")

	rr = doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations", testUser.ID, startBody("USD", "EUR", "0.9", previewToken))
	c.Assert(rr.Code, qt.Equals, http.StatusConflict)
	assertErrorCode(t, c, rr.Body.Bytes(), "currency_migration.restore_in_progress")
}

func TestCurrencyMigrations_LockMiddleware_BlocksCommodityWrites423(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newCurrencyMigrationParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Plant a running migration row + group lock signal.
	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
	cmReg := must.Must(params.FactorySet.CurrencyMigrationRegistryFactory.CreateUserRegistry(ctx))
	now := time.Now().UTC()
	mig := must.Must(cmReg.Create(ctx, models.CurrencyMigration{
		FromCurrency: models.Currency("USD"),
		ToCurrency:   models.Currency("EUR"),
		ExchangeRate: decimal.RequireFromString("0.9"),
		Status:       models.CurrencyMigrationStatusRunning,
		StartedAt:    &now,
	}))
	c.Assert(mig.ID, qt.Not(qt.Equals), "")

	// Try to PATCH a commodity — should be 423.
	rs := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
	commodities := must.Must(rs.CommodityRegistry.List(ctx))
	c.Assert(len(commodities) > 0, qt.IsTrue)
	target := commodities[0]

	rr := doJSONAPIRequest(t, handler, http.MethodPatch, "/api/v1/g/"+testGroup.Slug+"/commodities/"+target.ID, testUser.ID, map[string]any{
		"data": map[string]any{
			"id":   target.ID,
			"type": "commodities",
			"attributes": map[string]any{
				"name": "edited under lock",
			},
		},
	})
	c.Assert(rr.Code, qt.Equals, http.StatusLocked)
	body := rr.Body.Bytes()
	assertErrorCode(t, c, body, "currency_migration.locked")
	// Meta carries the in-flight migration's id and status so the FE
	// banner can deep-link without re-querying.
	assertErrorMeta(t, c, body, "migration_id", func(v any) bool { s, _ := v.(string); return s == mig.ID }, "expected migration_id == planted mig.ID")
	assertErrorMeta(t, c, body, "status", func(v any) bool { return v == "running" }, "expected status=running")
}

func TestCurrencyMigrations_LockOnRestoreCreate_Returns423(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newCurrencyMigrationParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)

	// Plant a running migration on the group.
	cmReg := must.Must(params.FactorySet.CurrencyMigrationRegistryFactory.CreateUserRegistry(ctx))
	now := time.Now().UTC()
	must.Must(cmReg.Create(ctx, models.CurrencyMigration{
		FromCurrency: models.Currency("USD"),
		ToCurrency:   models.Currency("EUR"),
		ExchangeRate: decimal.RequireFromString("0.9"),
		Status:       models.CurrencyMigrationStatusRunning,
		StartedAt:    &now,
	}))

	rs := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
	exp := must.Must(rs.ExportRegistry.Create(ctx, models.Export{
		Type:        models.ExportTypeFullDatabase,
		Description: "test export for restore",
		Status:      models.ExportStatusCompleted,
	}))

	rr := doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/exports/"+exp.ID+"/restores", testUser.ID, map[string]any{
		"data": map[string]any{
			"type": "restores",
			"attributes": map[string]any{
				"description": "blocked by migration",
				"options": map[string]any{
					"strategy": "full_replace",
				},
			},
		},
	})
	c.Assert(rr.Code, qt.Equals, http.StatusLocked)
	assertErrorCode(t, c, rr.Body.Bytes(), "currency_migration.locked")
}

func TestCurrencyMigrations_NonAdmin_Returns403(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newCurrencyMigrationParams()

	// Create a second user, add them to the group as a non-admin member.
	tenantID := testUser.TenantID
	memberTemplate := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Email:               "member@example.com",
		Name:                "Member User",
		IsActive:            true,
	}
	must.Assert(memberTemplate.SetPassword("Password123"))
	memberUser := must.Must(params.FactorySet.UserRegistry.Create(context.Background(), memberTemplate))
	must.Must(params.FactorySet.GroupMembershipRegistry.Create(context.Background(), models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		GroupID:             testGroup.ID,
		MemberUserID:        memberUser.ID,
		Role:                models.GroupRoleUser,
	}))

	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Each of the four endpoints must reject non-admins. Verify
	// preview / start / list / get all return 403.
	rr := doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations/preview", memberUser.ID, previewBody("USD", "EUR", "0.9"))
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf("preview"))

	rr = doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations", memberUser.ID, startBody("USD", "EUR", "0.9", "x"))
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf("start"))

	rr = doJSONAPIRequest(t, handler, http.MethodGet, "/api/v1/g/"+testGroup.Slug+"/currency-migrations", memberUser.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf("list"))

	rr = doJSONAPIRequest(t, handler, http.MethodGet, "/api/v1/g/"+testGroup.Slug+"/currency-migrations/some-id", memberUser.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf("get"))
}

func TestCurrencyMigrations_DailyCap_Returns429(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newCurrencyMigrationParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Plant 2 completed migrations today on this group.
	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
	cmReg := must.Must(params.FactorySet.CurrencyMigrationRegistryFactory.CreateUserRegistry(ctx))
	now := time.Now().UTC()
	for range 2 {
		// Memory registry doesn't enforce the partial unique index, so
		// status=completed rows can be inserted directly.
		op := must.Must(cmReg.Create(ctx, models.CurrencyMigration{
			FromCurrency: models.Currency("USD"),
			ToCurrency:   models.Currency("EUR"),
			ExchangeRate: decimal.RequireFromString("0.9"),
			Status:       models.CurrencyMigrationStatusCompleted,
			StartedAt:    &now,
			CompletedAt:  &now,
		}))
		// Memory Create may stamp Status from NewCurrencyMigrationFromUserInput;
		// re-update to enforce the completed status used by CompletedTodayForGroup.
		_ = op
		_ = cmReg.UpdateStatus(ctx, op.ID, registry.CurrencyMigrationStatusPatch{
			Status:      models.CurrencyMigrationStatusCompleted,
			CompletedAt: &now,
		})
	}

	// Third attempt → 429.
	rr := doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations/preview", testUser.ID, previewBody("USD", "EUR", "0.9"))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	previewToken := jsonPathString(t, rr.Body.Bytes(), "$.data.attributes.preview_token")

	rr = doJSONAPIRequest(t, handler, http.MethodPost, "/api/v1/g/"+testGroup.Slug+"/currency-migrations", testUser.ID, startBody("USD", "EUR", "0.9", previewToken))
	c.Assert(rr.Code, qt.Equals, http.StatusTooManyRequests)
	body := rr.Body.Bytes()
	assertErrorCode(t, c, body, "currency_migration.daily_cap_reached")
	// retry_after_seconds tells the FE when the cap window resets so
	// the Migrate CTA can stay disabled until then.
	assertErrorMeta(t, c, body, "retry_after_seconds", func(v any) bool {
		// json.Unmarshal decodes numbers into float64; accept any
		// positive value in the (0, 86400] range.
		if f, ok := v.(float64); ok {
			return f > 0 && f <= 24*60*60
		}
		return false
	}, "expected positive retry_after_seconds <= 86400")
}

// jsonPathString extracts a string value from a JSON body via jsonpath.
// Test-only helper; failing the t fatally on parse errors keeps the
// caller terse.
func jsonPathString(t *testing.T, body []byte, path string) string {
	t.Helper()
	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("jsonPathString: parse: %v\nbody=%s", err, string(body))
	}
	v, err := jsonpath.Read(parsed, path)
	if err != nil {
		t.Fatalf("jsonPathString(%s): %v\nbody=%s", path, err, string(body))
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("jsonPathString(%s): expected string, got %T (%v)", path, v, v)
	}
	return s
}
