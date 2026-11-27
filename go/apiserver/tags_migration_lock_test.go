package apiserver_test

import (
	"net/http"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
)

// migratingTagFixture seeds a tag of kind in a group whose currency
// migration is running, with the currency migration feature on.
func migratingTagFixture(c *qt.C, kind models.TagKind) (*models.User, *models.LocationGroup, *models.Tag, http.Handler) {
	c.Helper()

	params, testUser, testGroup := newCurrencyMigrationParams()
	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
	tag := must.Must(registrySet.TagRegistry.Create(ctx, models.Tag{
		Kind: kind, Slug: "kitchen", Label: "Kitchen", Color: models.TagColorBlue,
	}))
	now := time.Now().UTC()
	must.Must(registrySet.CurrencyMigrationRegistry.Create(ctx, models.CurrencyMigration{
		FromCurrency: models.Currency("USD"),
		ToCurrency:   models.Currency("EUR"),
		ExchangeRate: decimal.RequireFromString("0.9"),
		Status:       models.CurrencyMigrationStatusRunning,
		StartedAt:    &now,
	}))
	return testUser, testGroup, tag, apiserver.APIServer(params, &mockRestoreWorker{})
}

func renameTagBody(tag *models.Tag) *jsonapi.TagUpdateRequest {
	return &jsonapi.TagUpdateRequest{Data: &jsonapi.TagUpdateRequestDataWrapper{
		ID:         tag.ID,
		Type:       "tags",
		Attributes: jsonapi.TagUpdateRequestData{Label: "Kitchenware", Slug: "kitchenware"},
	}}
}

// Renaming or deleting an item tag rewrites the items that carry it, which
// waits for a running currency migration; the request is refused with the
// lock /commodities answers instead of waiting until it times out.
func TestItemTagWrites_DuringCurrencyMigration_Return423(t *testing.T) {
	c := qt.New(t)
	testUser, testGroup, tag, handler := migratingTagFixture(c, models.TagKindCommodity)
	tagURL := "/api/v1/g/" + testGroup.Slug + "/tags/" + tag.ID

	rr := tagsRequest(c, handler, http.MethodPatch, tagURL, testUser.ID, renameTagBody(tag))
	c.Assert(rr.Code, qt.Equals, http.StatusLocked)
	assertErrorCode(t, c, rr.Body.Bytes(), "currency_migration.locked")

	rr = tagsRequest(c, handler, http.MethodDelete, tagURL+"?force=true", testUser.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusLocked)
	assertErrorCode(t, c, rr.Body.Bytes(), "currency_migration.locked")

	rr = tagsRequest(c, handler, http.MethodGet, tagURL, testUser.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("reads are not locked"))
}

// A new label or color of an item tag writes the tag row alone, so a
// running currency migration does not hold it up; only a new slug rewrites
// the items (#3011).
func TestItemTagRelabel_DuringCurrencyMigration_Succeeds(t *testing.T) {
	c := qt.New(t)
	testUser, testGroup, tag, handler := migratingTagFixture(c, models.TagKindCommodity)
	tagURL := "/api/v1/g/" + testGroup.Slug + "/tags/" + tag.ID
	body := &jsonapi.TagUpdateRequest{Data: &jsonapi.TagUpdateRequestDataWrapper{
		ID:         tag.ID,
		Type:       "tags",
		Attributes: jsonapi.TagUpdateRequestData{Label: "Kitchenware", Slug: tag.Slug, Color: models.TagColorGreen},
	}}

	rr := tagsRequest(c, handler, http.MethodPatch, tagURL, testUser.ID, body)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.attributes.label"), "Kitchenware")
}

// A file tag touches no item, so a running currency migration does not hold
// it up.
func TestFileTagWrites_DuringCurrencyMigration_Succeed(t *testing.T) {
	c := qt.New(t)
	testUser, testGroup, tag, handler := migratingTagFixture(c, models.TagKindFile)
	tagURL := "/api/v1/g/" + testGroup.Slug + "/tags/" + tag.ID

	rr := tagsRequest(c, handler, http.MethodPatch, tagURL, testUser.ID, renameTagBody(tag))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	rr = tagsRequest(c, handler, http.MethodDelete, tagURL, testUser.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)
}
